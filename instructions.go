package main

const (
	opADD   = 0x0  // R type
	opADDI  = 0x8  // I type
	opADDIU = 0x9  // I type
	opADDU  = 0x0  // R type
	opAND   = 0x0  // R type
	opANDI  = 0xC  // I type
	opBEQ   = 0x4  // I type
	opBNE   = 0x5  // I type
	opDIV   = 0x0  // R type
	opDIVU  = 0x0  // R type
	opJ     = 0x2  // J type
	opJAL   = 0x3  // J type
	opJR    = 0x0  // R type
	opLB    = 0x20 // I type
	opLBU   = 0x24 // I type
	opLUI   = 0xF  // I type
	opLW    = 0x23 // I type
	opMFHI  = 0x0  // R type
	opMFLO  = 0x0  // R type
	opMULT  = 0x0  // R type
	opMULTU = 0x0  // R type
	opXOR   = 0x0  // R type
	opOR    = 0x0  // R type
	opORI   = 0xD  // I type
	opSB    = 0x28 // I type
	opSLT   = 0x0  // R type
	opSLTI  = 0xA  // I type
	opSLTIU = 0xB  // I type
	opSLTU  = 0x0  // R type
	opSLL   = 0x0  // R type
	opSRL   = 0x0  // R type
	opSRA   = 0x0  // R type
	opSUB   = 0x0  // R type
	opSUBU  = 0x0  // R type
	opSW    = 0x2B // I type
	opSWI   = 0x2F // I type
)

const (
	fnADD   = 0x20
	fnADDU  = 0x21
	fnAND   = 0x24
	fnDIV   = 0x1A
	fnDIVU  = 0x1B
	fnJR    = 0x08
	fnMFHI  = 0x10
	fnMFLO  = 0x12
	fnMULT  = 0x18
	fnMULTU = 0x19
	fnXOR   = 0x26
	fnOR    = 0x25
	fnSLT   = 0x2A
	fnSLTU  = 0x2B
	fnSLL   = 0x00
	fnSRL   = 0x02
	fnSRA   = 0x03
	fnSLLV  = 0x04
	fnSRLV  = 0x05
	fnSRAV  = 0x06
	fnSUB   = 0x22
	fnSUBU  = 0x23
)

func formRInstruction(opCode, rs, rt, rd, shift, funct int) uint32 {
	return (uint32(opCode) << 26) | uint32(rs<<21) | uint32(rt<<16) | uint32(rd<<11) | uint32(shift<<6) | uint32(funct)
}

func formIInstruction(opCode, rs, rt int, imm uint32) uint32 {
	return (uint32(opCode) << 26) | uint32(rs<<21) | uint32(rt<<16) | (imm & 0xFFFF)
}

func formJInstruction(opCode int, addr uint32) uint32 {
	return (uint32(opCode) << 26) | addr
}

func decodeInstruction(instr uint32) (op, x, y, z int, imm uint32, fn int) {
	//last 6 bits are the op code and determine how to read the rest of the instruction
	op = int(instr >> 26)
	if op == 0x0 {
		//R-type instruction where order is: op, rs, rt, rd, shift, fn
		//rd is z, rs is x, rt is y
		x = int((instr >> 21) & 0x1F)
		y = int((instr >> 16) & 0x1F)
		z = int((instr >> 11) & 0x1F)
		imm = (instr >> 6) & 0x1F //doubles as shift amount
		fn = int(instr & 0x3F)
		return
	} else if op == opJ || op == opJAL {
		//J-type instruction where the order is op, addr
		imm = instr & 0x03FFFFFF
		return
	} else {
		//I-type instruction where order is: op, rs, rt, immediate
		//rs is z, rt is x
		x = int((instr >> 16) & 0x1F)
		z = int((instr >> 21) & 0x1F)
		imm = instr & 0xFFFF
		return
	}
}
