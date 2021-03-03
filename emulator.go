package main

import (
	"fmt"
)

/**
 * Emulator
 * The emulator is performance-oriented and does not keep track of what it does (compared to MiSaSiM)
 * The emulator reads machine code and is of the Von-Nuemann model (meaning that instructions and data share the same
 *   memory space)
 * Because it reads from the actual memory, programs can dynamically edit the program memory which is something
 *   MiSaSiM does not support.
 * To allow for massive lookup tables to have improved performance, the emulator uses caching
 */

const (
	eUninitializedMemoryAccess int = iota
	eUninitializedRegisterAccess
	eRuntimeLimitExceeded
	eInvalidInstruction
	eIllegalRegisterWrite
	eErrorLimitReached
	eShiftOverflow
	eHiLoUninitializedAccess
	eSoftwareInterruptParameter
	eInvalidSoftwareInterrupt
	eSoftwareInterruptParameterValue
	eNoAnswerReported
)

type MemoryPage struct {
	startAddr   uint32
	memory      []uint32 //is static-sized to the length of a page (4KB)
	initialized []uint32 //bits are set depending on if a word in memory has been initialized before reading
}

type SystemMemory map[uint32]MemoryPage

type RuntimeError struct {
	EType   int
	Message string
}

type BranchInfo struct {
	TotalCount  uint32
	BranchCount uint32
}

type instance struct {
	memory       SystemMemory
	branchInfo   map[uint32]BranchInfo
	pc           uint32
	regs         [32]uint32
	regInit      uint32
	hiLoFilled   bool
	hi, lo       uint32
	iCache       MemoryPage
	dCache       MemoryPage
	dMissed      bool
	di           uint32
	runtimeLimit uint32
	swiContext   interface{}

	errors []RuntimeError //keeping the errors to return from emulation
}

type EmulationResult struct {
	Memory         SystemMemory
	Registers      [32]uint32
	RegInit        uint32
	DI             uint32
	SWIContext     interface{}
	BranchAnalysis map[uint32]BranchInfo
	Errors         []RuntimeError
}

/**
 * The system memory is expressed as a map to improve emulator performance of systems that have spread out memory locations
 *
 * The key is the upper 20 bits of the memory address
 * To improve emulator performance, the two memory pages will be cached
 * 		One memory page will be for data, the other for instructions
 * 		The instruction cache will be replaced if the instruction decoder has a cache miss once.
 * 		The data cache will be replaced after two consecutive cache misses.
 */

func addToSystemMemory(img *MemoryImage, mem map[uint32]MemoryPage) map[uint32]MemoryPage {
	currentPage := uint32(0xFFFFFFFF) //an invalid page to guarantee that the change of page code executes
	for i := 0; len(img.memory)*4 > i; i += 4 {
		if ((img.startingAddr+uint32(i/4))&0xFFFFF000)>>12 != currentPage {
			//change of pages
			currentPage = ((img.startingAddr + uint32(i/4)) & 0xFFFFF000) >> 12

			//checking if the map currently contains this page
			_, ok := mem[currentPage]
			if !ok {
				mem[currentPage] = MemoryPage{
					startAddr:   currentPage << 12,
					memory:      make([]uint32, 1024),
					initialized: make([]uint32, 32),
				}
			}
		}
		mem[currentPage].memory[((img.startingAddr+uint32(i))%4096)/4] = img.memory[i/4]
		mem[currentPage].initialized[((img.startingAddr+uint32(i))%4096)/128] =
			mem[currentPage].initialized[((img.startingAddr+uint32(i))%4096)/128] |
				0x1<<((img.startingAddr+uint32(i))/4%32) //setting this word to "initialized"
	}

	return mem
}

//accepts formatting
func (inst *instance) reportError(eType int, format string, fArgs ...interface{}) {
	eStr := fmt.Sprintf("ERROR: pc=0x%X di=%d message=%s", inst.pc, inst.di+1, fmt.Sprintf(format, fArgs...))
	inst.errors = append(inst.errors, RuntimeError{
		EType:   eType,
		Message: eStr,
	})
}

func (m *SystemMemory) memRead(addr uint32) (uint32, bool) {
	page, ok := (*m)[addr>>12]
	if !ok {
		return 0, false
	}

	if (page.initialized[(addr%4096)/128]>>((addr%4096)/4%32))&0x1 != 0x1 {
		//not initialized
		return 0, false
	}

	return page.memory[addr/4%1024], true
}

func (r *EmulationResult) regRead(reg int) (uint32, bool) {
	if (r.RegInit>>reg)&0x1 != 0x1 {
		return 0, false
	}

	return r.Registers[reg], true
}

//access functions

func (inst *instance) memAccess(addr uint32, isInstr bool) (uint32, bool) {
	//checking cache first
	if addr>>12 == inst.iCache.startAddr>>12 {
		//from instruction cache, checking if the value has been initialized
		if (inst.iCache.initialized[(addr%4096)/128]>>((addr%4096)/4%32))&0x1 != 0x1 {
			//not initialized
			inst.reportError(eUninitializedMemoryAccess, "0x%X (%d) was accessed before it was initialized", addr, addr)
			return 0, false
		}

		return inst.iCache.memory[addr/4%1024], true
	} else if addr>>12 == inst.dCache.startAddr>>12 {
		//from data cache, checking if the value has been initialized
		if (inst.dCache.initialized[(addr%4096)/128]>>((addr%4096)/4%32))&0x1 != 0x1 {
			//not initialized
			inst.reportError(eUninitializedMemoryAccess, "0x%X (%d) was accessed before it was initialized", addr, addr)
			return 0, false
		}

		inst.dMissed = false
		return inst.dCache.memory[addr/4%1024], true
	}

	page, ok := inst.memory[addr>>12]
	if !ok {
		inst.reportError(eUninitializedMemoryAccess, "0x%X (%d) was accessed before it was initialized", addr, addr)
		return 0, false
	}

	if (page.initialized[(addr%4096)/128]>>((addr%4096)/4%32))&0x1 != 0x1 {
		//not initialized
		inst.reportError(eUninitializedMemoryAccess, "0x%X (%d) was accessed before it was initialized", addr, addr)
		return 0, false
	}

	if isInstr {
		//cannot tolerate cache misses
		inst.iCache = page
	} else if inst.dMissed == true {
		//already missed data cache once, needs to flush
		inst.dCache = page
	} else {
		inst.dMissed = true
	}

	return page.memory[addr/4%1024], true
}

//mask and data should be shifted as per the address requirements before this function call
func (inst *instance) memWrite(addr, data, mask uint32) {
	if addr>>12 == inst.iCache.startAddr>>12 {
		//to instruction cache
		inst.iCache.memory[addr/4%1024] = (data & mask) |
			(inst.iCache.memory[addr/4%1024] & (mask ^ 0xFFFFFFFF))

		inst.iCache.initialized[(addr%4096)/128] |= 0x1 << ((addr % 4096) / 4 % 32)

		//instruction cache is not flushed from a write operation
		return
	} else if addr>>12 == inst.dCache.startAddr>>12 {
		//to data cache
		inst.dCache.memory[addr/4%1024] = (data & mask) |
			(inst.dCache.memory[addr/4%1024] & (mask ^ 0xFFFFFFFF))

		inst.dCache.initialized[(addr%4096)/128] |= 0x1 << ((addr % 4096) / 4 % 32)
		inst.dMissed = false
		return
	}

	//testing if the page exists yet
	page, ok := inst.memory[addr>>12]
	if !ok {
		//need to create the page
		page = MemoryPage{
			startAddr:   addr & 0xFFFFF000,
			memory:      make([]uint32, 1024),
			initialized: make([]uint32, 32),
		}
		inst.memory[addr>>12] = page
	}

	page.memory[addr/4%1024] = (data & mask) | (page.memory[addr/4%1024] & (mask ^ 0xFFFFFFFF))

	page.initialized[(addr%4096)/128] |= 0x1 << ((addr % 4096) / 4 % 32)
}

func (inst *instance) regInitialized(reg int) bool {
	return (inst.regInit>>reg)&0x1 == 0x1
}

func (inst *instance) regAccess(reg int) uint32 {
	if (inst.regInit>>reg)&0x1 != 0x1 {
		inst.reportError(eUninitializedRegisterAccess, "$%d was accessed before it was initialized", reg)
		return 0
	}

	return inst.regs[reg]
}

func (inst *instance) regWrite(reg int, data uint32) {
	//setting initialized bit
	if reg == 0 {
		inst.reportError(eIllegalRegisterWrite, "$0 is immutable and cannot be written to")
		return
	}

	inst.regInit = inst.regInit | (0x1 << reg)
	inst.regs[reg] = data
}

/**
 * Emulation entry function
 * 	Is multithreading friendly
 */
func Emulate(startAddr uint32, mem SystemMemory, limit uint32, eTol int) EmulationResult {
	inst := new(instance)
	inst.memory = mem
	inst.regs[0] = 0           //reg 0 is an immutable zero.
	inst.regs[31] = 0xFFFFFFFF //the program exit pc value
	inst.regs[29] = 0x00100000 //the stack pointer register
	inst.regInit = 0x1 | 0x1<<29 | 0x1<<31
	inst.pc = startAddr & 0xFFFFFFFC //protection so that it always has the correct byte alignment
	inst.hi = 0
	inst.lo = 0
	inst.hiLoFilled = false
	inst.runtimeLimit = limit
	inst.di = 0

	//initializing instruction cache

	for true {
		if inst.pc == 0xFFFFFFFF || len(inst.errors) >= eTol || inst.di > limit {
			if len(inst.errors) >= eTol {
				inst.reportError(eErrorLimitReached, "maximum of %d errors has been exceeded, stopping emulation", eTol)
			} else if inst.di > limit {
				inst.reportError(eRuntimeLimitExceeded, "maximum runtime instruction count of %d exceeded", limit)
			}
			break
		}

		//decode instruction
		instr, ok := inst.memAccess(inst.pc, true)
		if !ok {
			//error already reported
			inst.pc += 4
			inst.di++
			continue
		}

		op, x, y, z, imm, fn := decodeInstruction(instr)

		if instr == 0 {
			//no-op, so do nothing
		} else if op == 0x0 {
			//R-type instruction where fn is the operation to perform
			inst.executeRType(x, y, z, fn, imm)
		} else if op == opJ || op == opJAL {
			inst.executeJType(op, imm)
		} else {
			inst.executeIType(op, x, z, imm)
		}

		inst.di++
		inst.pc += 4
	}

	return EmulationResult{
		Memory:         inst.memory,
		Registers:      inst.regs,
		DI:             inst.di,
		SWIContext:     inst.swiContext,
		BranchAnalysis: inst.branchInfo,
		Errors:         inst.errors,
		RegInit:        inst.regInit,
	}
}

func (inst *instance) executeRType(x, y, z, fn int, shift uint32) {
	switch fn {
	case fnADD:
		inst.regWrite(z, uint32(int32(inst.regAccess(x))+int32(inst.regAccess(y))))
		break
	case fnADDU:
		inst.regWrite(z, inst.regAccess(x)+inst.regAccess(y))
		break
	case fnAND:
		inst.regWrite(z, inst.regAccess(x)&inst.regAccess(y))
		break
	case fnDIV:
		inst.lo = uint32(int32(inst.regAccess(x)) / int32(inst.regAccess(y)))
		inst.hi = uint32(int32(inst.regAccess(x)) % int32(inst.regAccess(y)))
		inst.hiLoFilled = true
		break
	case fnDIVU:
		inst.lo = inst.regAccess(x) / inst.regAccess(y)
		inst.hi = inst.regAccess(x) % inst.regAccess(y)
		inst.hiLoFilled = true
		break
	case fnJR:
		inst.pc = inst.regAccess(x) - 4 // the minus four is to account for the pc increment
		break
	case fnMFHI:
		if !inst.hiLoFilled {
			inst.reportError(eHiLoUninitializedAccess, "mfhi used on uninitialized result")
		}
		inst.regWrite(z, inst.hi)
		break
	case fnMFLO:
		if !inst.hiLoFilled {
			inst.reportError(eHiLoUninitializedAccess, "mflo used on uninitialized result")
		}
		inst.regWrite(z, inst.lo)
		break
	case fnMULT:
		res := int64(inst.regAccess(x)) * int64(inst.regAccess(y))
		inst.hi = uint32(res >> 32)
		inst.lo = uint32(res)
		inst.hiLoFilled = true
		break
	case fnMULTU:
		res := uint64(inst.regAccess(x)) * uint64(inst.regAccess(y))
		inst.hi = uint32(res >> 32)
		inst.lo = uint32(res)
		inst.hiLoFilled = true
		break
	case fnXOR:
		inst.regWrite(z, inst.regAccess(x)^inst.regAccess(y))
		break
	case fnOR:
		inst.regWrite(z, inst.regAccess(x)|inst.regAccess(y))
		break
	case fnSLT:
		if int32(inst.regAccess(x)) < int32(inst.regAccess(y)) {
			inst.regWrite(z, 1)
		} else {
			inst.regWrite(z, 0)
		}
		break
	case fnSLTU:
		if inst.regAccess(x) < inst.regAccess(y) {
			inst.regWrite(z, 1)
		} else {
			inst.regWrite(z, 0)
		}
		break
	case fnSLL:
		inst.regWrite(z, inst.regAccess(x)<<shift)
		break
	case fnSRL:
		inst.regWrite(z, inst.regAccess(x)>>shift)
		break
	case fnSRA:
		inst.regWrite(z, uint32(int32(inst.regAccess(x))>>shift))
		break
	case fnSLLV:
		amt := inst.regAccess(y)
		if amt > 31 {
			inst.reportError(eShiftOverflow, "%d is larger than the maximum shift amount of 31", amt)
		}
		inst.regWrite(z, inst.regAccess(x)<<(amt&0x1F))
		break
	case fnSRLV:
		amt := inst.regAccess(y)
		if amt > 31 {
			inst.reportError(eShiftOverflow, "%d is larger than the maximum shift amount of 31", amt)
		}
		inst.regWrite(z, inst.regAccess(x)>>(amt&0x1F))
		break
	case fnSRAV:
		amt := inst.regAccess(y)
		if amt > 31 {
			inst.reportError(eShiftOverflow, "%d is larger than the maximum shift amount of 31", amt)
		}
		inst.regWrite(z, uint32(int32(inst.regAccess(x))>>(amt&0x1F)))
		break
	case fnSUB:
		inst.regWrite(z, uint32(int32(inst.regAccess(x))-int32(inst.regAccess(y))))
		break
	case fnSUBU:
		inst.regWrite(z, inst.regAccess(x)-inst.regAccess(y))
		break
	default:
		inst.reportError(eInvalidInstruction, "%X is not a valid function for an R-type instruction", fn)
	}
}

func (inst *instance) executeIType(op, x, z int, imm uint32) {
	switch op {
	case opADDI:
		//sign extend the immediate
		imm = uint32(int32(imm<<16) >> 16) //uses arithmetic shifting to copy the sign
		inst.regWrite(z, inst.regAccess(x)+imm)
		break
	case opADDIU:
		imm = uint32(int32(imm<<16) >> 16) //uses arithmetic shifting to copy the sign because it isn't actually unsigned (wtf mips..)
		inst.regWrite(z, inst.regAccess(x)+imm)
		break
	case opANDI:
		inst.regWrite(z, inst.regAccess(x)&imm)
		break
	case opBEQ:
		if inst.regAccess(z) == inst.regAccess(x) {
			//branch to the address immediate * 4
			inst.pc = imm*4 - 4 //the - 4 is to account for the pc increment in the main loop
		}
		break
	case opBNE:
		if inst.regAccess(z) != inst.regAccess(x) {
			//branch to the address immediate * 4
			inst.pc = imm*4 - 4 //the - 4 is to account for the pc increment in the main loop
		}
		break
	case opLB:
		a := inst.regAccess(x) + imm
		v, _ := inst.memAccess(a, false)
		v = v >> ((a % 4) * 8)
		//sign extending the byte
		v = uint32(int32((v&0xFF)<<24) >> 24)
		inst.regWrite(z, v)
		break
	case opLBU:
		a := inst.regAccess(x) + imm
		v, _ := inst.memAccess(a, false)
		v = v >> ((a % 4) * 8)
		inst.regWrite(z, v&0xFF)
		break
	case opLW:
		a := inst.regAccess(x) + imm
		v, _ := inst.memAccess(a, false)
		inst.regWrite(z, v)
		break
	case opLUI:
		inst.regWrite(z, imm<<16)
		break
	case opORI:
		inst.regWrite(z, inst.regAccess(x)|imm)
		break
	case opSB:
		a := inst.regAccess(x) + imm
		b := inst.regAccess(z) & 0xFF
		b = b << ((a % 4) * 8)
		inst.memWrite(a, b, 0xFF<<((a%4)*8))
		break
	case opSLTI:
		if int32(inst.regAccess(x)) < int32(imm) {
			inst.regWrite(z, 1)
		} else {
			inst.regWrite(z, 0)
		}
		break
	case opSLTIU:
		if inst.regAccess(x) < imm {
			inst.regWrite(z, 1)
		} else {
			inst.regWrite(z, 0)
		}
		break
	case opSW:
		a := inst.regAccess(x) + imm
		inst.memWrite(a, inst.regAccess(z), 0xFFFFFFFF)
		break
	case opSWI:
		inst.dispatchSoftwareInterrupt(int(imm))
		break
	default:
		inst.reportError(eInvalidInstruction, "%X is not a valid opcode for an instruction", op)
	}
}

func (inst *instance) executeJType(op int, imm uint32) {
	if op == opJ {
		inst.pc = imm*4 - 4 //accounting for the increment
	} else if op == opJAL {
		inst.regWrite(31, inst.pc+8) //there should be a nop instruction following the jal
		inst.pc = imm*4 - 4          //accounting for the increment
	}
}

func decodeErrorCode(iCode int) string {
	/**
	eUninitializedMemoryAccess int = iota
	eUninitializedRegisterAccess
	eRuntimeLimitExceeded
	eInvalidInstruction
	eIllegalRegisterWrite
	eErrorLimitReached
	eShiftOverflow
	eHiLoUninitializedAccess
	eSoftwareInterruptParameter
	eInvalidSoftwareInterrupt
	eSoftwareInterruptParameterValue
	eNoAnswerReported
	*/

	switch iCode {
	case eUninitializedMemoryAccess:
		return "eUninitializedMemoryAccess"
	case eUninitializedRegisterAccess:
		return "eUninitializedRegisterAccess"
	case eRuntimeLimitExceeded:
		return "eRuntimeLimitExceeded"
	case eInvalidInstruction:
		return "eInvalidInstruction"
	case eIllegalRegisterWrite:
		return "eIllegalRegisterWrite"
	case eErrorLimitReached:
		return "eErrorLimitReached"
	case eShiftOverflow:
		return "eShiftOverflow"
	case eHiLoUninitializedAccess:
		return "eHiLoUninitializedAccess"
	case eSoftwareInterruptParameter:
		return "eSoftwareInterruptParameter"
	case eInvalidSoftwareInterrupt:
		return "eInvalidSoftwareInterrupt"
	case eSoftwareInterruptParameterValue:
		return "eSoftwareInterruptParameterValue"
	case eNoAnswerReported:
		return "eNoAnswerReported"
	}

	return "genericError"
}
