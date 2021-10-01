package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

/**
 * Assembler
 * This file contains the entire assembler, minus instruction declarations and forming (instructions.go)
 * Compared to MiSaSiM, the assembler gives more detailed errors and is strict, meaning that it treats all
 * warnings as errors (for example, immediate value overflows).
 *
 * Also, compared to MiSaSiM, the assembler generates machine code that **should** be to MIPS 1.0 spec
 */

type MemoryImage struct {
	startingAddr uint32
	memory       []uint32
}

type AssemblySettings struct {
	TextStart uint32 //must be a multiple of 4
	DataStart uint32 //must be a multiple of 4
}

type InputLine struct {
	Contents   string
	LineNumber int
}

const (
	assemExtractNone int = iota
	assemExtractData
	assemExtractText
)

var numErrors = 0

func assemblyReportError(line InputLine, eText string) {
	if len(line.Contents) > 64 {
		//shortening the line
		line.Contents = line.Contents[:64]
	}

	fullText := fmt.Sprintf("%d (%s): Error: %s", line.LineNumber, line.Contents, eText)
	fmt.Println(fullText)
	numErrors++
}

func insertMemoryValue(addr, value uint32, mem *MemoryImage) {
	//assuming value has already been masked

	for addr >= mem.startingAddr+uint32(len(mem.memory))*4 {
		//expanding memory, assuming memory is contiguous for the assembler
		mem.memory = append(mem.memory, 0)
	}

	//inserting the value
	prev := mem.memory[(addr-mem.startingAddr)/4]
	prev = prev | (value << ((addr % 4) * 8))
	mem.memory[(addr-mem.startingAddr)/4] = prev
}

func getLiteralValueFull(s string, labels map[string]uint32, isSignedImm bool) (uint32, error) {
	//literals in this case can be labels as well
	s = strings.Trim(s, " \t")

	if s == "" {
		return 0, fmt.Errorf("expected a literal, got nothing")
	}
	if s[0] == '-' || unicode.IsDigit(rune(s[0])) {
		//is a number

		s = strings.ToLower(s)
		if strings.Index(s, "0x") == 0 {
			//the number is hex
			s = s[2:]
			var ret uint32 = 0
			for i := range s {
				c := s[i]
				if c <= '9' && c >= '0' {
					ret = (ret << 4) | uint32(c-'0')
				} else if c <= 'f' && c >= 'a' {
					ret = (ret << 4) | (uint32(c-'a') + 10)
				} else {
					//invalid character in a hex number
					return 0, fmt.Errorf("\"%s\" is not a valid hexadecimal number", s)
				}
			}
			return ret, nil
		}

		//the number is base 10
		if s[0] == '-' {
			//the number is negative
			s = s[1:]
			var ret uint32
			for i := range s {
				c := s[i]
				if c <= '9' && c >= '0' {
					if ret > 0xE666666 {
						//an overflow will occur
						return 0, fmt.Errorf("\"-%s\" has magnitude too large to store in 32 bits", s)
					}
					ret = (ret * 10) + uint32(c-'0')
				} else {
					//invalid character in the number
					return 0, fmt.Errorf("\"-%s\" is not a valid integer number", s)
				}
			}

			return ret ^ 0xFFFFFFFF + 1, nil
		}

		var ret uint32
		for i := range s {
			c := s[i]
			if c <= '9' && c >= '0' {
				if ret > 0x19999999 {
					//an overflow will occur
					return 0, fmt.Errorf("\"%s\" has magnitude too large to store in 32 bits", s)
				}
				ret = (ret * 10) + uint32(c-'0')
			} else {
				//invalid character in the number
				return 0, fmt.Errorf("\"%s\" is not a valid integer number", s)
			}
		}

		return ret, nil
	} else {
		//must be a label
		v, ok := labels[s]
		if !ok {
			return 0, fmt.Errorf("unresolved label \"%s\"", s)
		}
		if isSignedImm {
			//return 0, fmt.Errorf("should not use labels in sign-extending immediate values")
		}
		return v, nil
	}
}

func getLiteralValue(s string, labels map[string]uint32) (uint32, error) {
	return getLiteralValueFull(s, labels, false)
}

func assembleData(lines []InputLine, settings AssemblySettings) (*MemoryImage, map[string]uint32) {
	//the map returned is a map of generated labels and their memory address

	//data types are as follows:
	// * .byte 		: one byte
	// * .halfword 	: two bytes
	// * .word 		: four bytes
	// * .space		: a specified number of bytes
	// * .alloc		: a specified number of words

	//general format: LabelName: .dataType value

	retMem := new(MemoryImage)
	labels := make(map[string]uint32)

	currentAddr := settings.DataStart - 1
	retMem.startingAddr = settings.DataStart

	for _, l := range lines {
		line := l.Contents

		//first removing comments from the line
		if strings.Contains(line, "#") {
			line = line[:strings.Index(line, "#")]
			line = strings.Trim(line, " \t")
		}

		//ignoring empty lines
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			//invalid syntax, should have at least three terms
			assemblyReportError(l, "data allocations must have at least 3 terms, expected "+
				"\"LabelName: .dataType value\". Got: \""+line+"\"")
			continue
		}

		if fields[0][len(fields[0])-1] != ':' {
			//no colon following a label declaration
			assemblyReportError(l, "data allocation labels need to be followed by a colon. Expected "+
				"\"LabelName: .dataType value\". Got: \""+line+"\"")
		}

		fields[0] = strings.Trim(fields[0], ": \t")

		switch strings.ToLower(fields[1]) {
		case ".byte":
			//merging all other fields together to prepare for comma delimited list
			dataMerged := strings.Join(fields[2:], "")
			dataMerged = strings.ReplaceAll(dataMerged, " ", "")
			values := strings.Split(dataMerged, ",")

			labels[fields[0]] = currentAddr + 1
			for _, literal := range values {
				v, e := getLiteralValue(literal, labels)
				if e != nil {
					assemblyReportError(l, e.Error()) //no need to skip the rest of the lines
				}

				currentAddr++
				if v&0xFFFFFF00 != 0xFFFFFF00 && v&0xFFFFFF00 != 0x0 {
					//overflow
					assemblyReportError(l, "\""+literal+"\" overflows a byte")
				}
				insertMemoryValue(currentAddr, v&0xFF, retMem)
			}
			break
		case ".halfword":
			//merging all other fields together to prepare for comma delimited list
			dataMerged := strings.Join(fields[2:], "")
			dataMerged = strings.ReplaceAll(dataMerged, " ", "")
			values := strings.Split(dataMerged, ",")

			labels[fields[0]] = (currentAddr + 2) & 0xFFFFFFFE //accounts for byte alignment
			for _, literal := range values {
				v, e := getLiteralValue(literal, labels)
				if e != nil {
					assemblyReportError(l, e.Error()) //no need to skip the rest of the lines
				}

				currentAddr += (currentAddr + 2) & 0xFFFFFFFE
				if v&0xFFFF0000 != 0xFFFF0000 && v&0xFFFF0000 != 0x0 {
					//overflow
					assemblyReportError(l, "\""+literal+"\" overflows a half word")
				}
				insertMemoryValue(currentAddr, v&0xFFFF, retMem)
			}

			break
		case ".word":
			//merging all other fields together to prepare for comma delimited list
			dataMerged := strings.Join(fields[2:], "")
			dataMerged = strings.ReplaceAll(dataMerged, " ", "")
			values := strings.Split(dataMerged, ",")

			labels[fields[0]] = (currentAddr + 4) & 0xFFFFFFFC //accounts for byte alignment
			for _, literal := range values {
				v, e := getLiteralValue(literal, labels)
				if e != nil {
					assemblyReportError(l, e.Error()) //no need to skip the rest of the lines
				}

				currentAddr += (currentAddr + 4) & 0xFFFFFFFC
				insertMemoryValue(currentAddr, v, retMem)
			}

			break
		case ".space":
			currentAddr++
			labels[fields[0]] = currentAddr

			v, e := getLiteralValue(fields[2], labels)
			if e != nil {
				assemblyReportError(l, e.Error())
				break
			}
			if v >= 65536*4 {
				assemblyReportError(l, "allocations larger than 256KiB are prohibited")
				break
			}

			for endAddr := currentAddr + v; endAddr > currentAddr; currentAddr++ {
				insertMemoryValue(currentAddr, 0, retMem)
			}

			currentAddr -= 1 //a lazy way of accounting for the one extra time it increments currentAddr

			break
		case ".alloc":
			currentAddr = (currentAddr + 4) & 0xFFFFFFFC //accounts for byte alignment
			labels[fields[0]] = currentAddr

			v, e := getLiteralValue(fields[2], labels)
			if e != nil {
				assemblyReportError(l, e.Error())
				break
			}
			if v >= 65536 {
				assemblyReportError(l, "allocations larger than 256KiB are prohibited")
				break
			}

			for endAddr := currentAddr + v*4; endAddr > currentAddr; currentAddr += 4 {
				insertMemoryValue(currentAddr, 0, retMem)
			}

			currentAddr -= 4 //a lazy way of accounting for the one extra time it increments currentAddr

			break
		default:
			assemblyReportError(l, "invalid data type. Valid data types are"+
				" .byte, .halfword, .word, .space, and .alloc")
			labels[fields[0]] = currentAddr //does this to prevent future errors in text assembly
		}
	}

	return retMem, labels
}

func extractTextLabels(lines []InputLine, settings AssemblySettings, labels map[string]uint32) map[string]uint32 {
	currentAddr := settings.TextStart

	for _, l := range lines {
		noComment := l.Contents
		if strings.Contains(noComment, "#") {
			noComment = noComment[:strings.Index(noComment, "#")]
		}
		noLabel := noComment
		if strings.Contains(noLabel, ":") {
			noLabel = noLabel[strings.Index(noLabel, ":")+1:]
		}
		noLabel = strings.Trim(noLabel, " \t")

		if noLabel == "" && strings.Contains(noComment, ":") {
			//label on an empty line, not allowed
			assemblyReportError(l, "cannot declare labels on lines without assembly operations")
		}

		if strings.Contains(noComment, ":") {
			//there is a label
			labelName := noComment[:strings.Index(noComment, ":")]
			labelName = strings.Trim(labelName, " \t")
			_, ok := labels[labelName]
			if ok {
				//label already declared, error
				assemblyReportError(l, "label \""+labelName+"\" already declared")
			}

			//even if the error is thrown, the label is to the last one because the program will never be run
			labels[labelName] = currentAddr
		}

		if noLabel != "" {
			currentAddr += 4

			//If the instruction is JAL, then must add an additional 4
			if strings.Index(strings.ToLower(noLabel), "jal") == 0 {
				currentAddr += 4
			}
		}

	}

	return labels
}

func getRegFromString(s string, line InputLine) (int, bool) {
	if len(s) == 0 {
		assemblyReportError(line, "missing register, cannot omit registers")
		return 0, false
	}

	if s[0] != '$' && s[0] != 't' {
		assemblyReportError(line, "registers are marked with a preceding '$' or 't'")
		return 0, false
	}

	v, e := strconv.Atoi(s[1:])
	if e != nil {
		assemblyReportError(line, "the specified register \""+s+"\" is not a valid numeric register")
		return 0, false
	}

	if v < 0 || v > 31 {
		assemblyReportError(line, "invalid register. Registers are between $0 and $31")
		return 0, false
	}

	return v, true
}

func extractRTypeInfo(fields []string, line InputLine, num int) ([3]int, bool) {
	if len(fields) != num {
		//invalid format
		if num == 3 {
			assemblyReportError(line, "this register-type instruction must have 3 registers in the form \"opcode $1, $2, $3\"")
		} else if num == 2 {
			assemblyReportError(line, "this register-type instruction must have 2 registers in the form \"opcode $1, $2\"")
		} else {
			assemblyReportError(line, "this register-type instruction must have 1 register in the form \"opcode $1\"")
		}
		return [3]int{}, false
	}

	var ret [3]int
	for i := 0; num > i; i++ {
		if len(fields[i]) == 0 {
			assemblyReportError(line, "missing register, cannot omit registers")
			return ret, false
		}

		if fields[i][0] != '$' && fields[i][0] != 't' {
			assemblyReportError(line, "registers are marked with a preceding '$' or 't'")
			return ret, false
		}

		v, e := strconv.Atoi(fields[i][1:])
		if e != nil {
			assemblyReportError(line, "the specified register \""+fields[i]+"\" is not a valid numeric register")
			return ret, false
		}

		if v < 0 || v > 31 {
			assemblyReportError(line, "invalid register. Registers are between $0 and $31")
			return ret, false
		}

		ret[i] = v
	}

	return ret, true
}

func extractStandardITypeInfo(fields []string, line InputLine, labels map[string]uint32, maxMask uint32, isSignedImm bool) ([2]int, uint32, bool) {
	if len(fields) != 3 {
		//invalid format
		assemblyReportError(line, "immediate-type instructions must have 2 registers and one immediate"+
			" in the form \"opcode $1, $2, [value]\"")
		return [2]int{}, 0, false
	}

	var ret [2]int
	for i := 0; 2 > i; i++ {
		v, ok := getRegFromString(fields[i], line)
		if !ok {
			return [2]int{}, 0, false
		}

		ret[i] = v
	}

	v, e := getLiteralValueFull(fields[2], labels, isSignedImm)
	if e != nil {
		assemblyReportError(line, e.Error())
		return ret, 0, false
	}
	if (v&maxMask) != maxMask && (v&maxMask) != 0x0 {
		//overflow
		assemblyReportError(line, "immediate value does not fit into 16 bits")
		return ret, 0, false
	}

	return ret, v & 0xFFFF, true
}

func extractSpecialITypeInfo(fields []string, line InputLine, labels map[string]uint32) ([2]int, uint32, bool) {
	//form is opcode $1, literal($2)

	if len(fields) != 2 {
		assemblyReportError(line, "invalid format. This instruction requires the format \"opcode $1, literal($2)\"")
		return [2]int{}, 0, false
	}

	var ret [2]int

	v, ok := getRegFromString(fields[0], line)
	if !ok {
		return ret, 0, false
	}
	ret[0] = v

	//getting the second register in the parenthesis
	if !strings.Contains(fields[1], "(") || !strings.Contains(fields[1], ")") {
		assemblyReportError(line, "invalid format, missing parenthesis-wrapped register."+
			" This instruction requires the format \"opcode $1, literal($2)\"")
		return ret, 0, false
	}

	secondReg := fields[1][strings.Index(fields[1], "(")+1 : strings.Index(fields[1], ")")]
	v, ok = getRegFromString(secondReg, line)
	if !ok {
		return ret, 0, false
	}

	ret[1] = v

	//getting literal
	literal := fields[1][:strings.Index(fields[1], "(")]
	lv, e := getLiteralValue(literal, labels)
	if e != nil {
		assemblyReportError(line, e.Error())
		return ret, 0, false
	}
	return ret, lv, true
}

func extractLUIInfo(fields []string, line InputLine, labels map[string]uint32) (int, uint32, bool) {
	if len(fields) != 2 {
		//invalid format
		assemblyReportError(line, "LUI instructions must have 1 register and one immediate"+
			" in the form \"lui $1, [value]\"")
		return 0, 0, false
	}

	r, ok := getRegFromString(fields[0], line)
	if !ok {
		return 0, 0, false
	}

	v, e := getLiteralValue(fields[1], labels)
	if e != nil {
		assemblyReportError(line, e.Error())
		return 0, 0, false
	}
	if (v&0xFFFF0000) != 0xFFFF0000 && (v&0xFFFF0000) != 0x0 {
		//overflow
		assemblyReportError(line, "immediate value does not fit into 16 bits")
		return 0, 0, false
	}

	return r, v, true
}

func assembleText(lines []InputLine, settings AssemblySettings, labels map[string]uint32) (*MemoryImage, map[uint32]InputLine) {
	currentAddr := settings.TextStart
	ret := new(MemoryImage)
	ret.startingAddr = settings.TextStart
	lineRet := make(map[uint32]InputLine)

	for _, l := range lines {
		noComment := l.Contents
		if strings.Contains(noComment, "#") {
			noComment = noComment[:strings.Index(noComment, "#")]
		}
		noLabel := noComment
		if strings.Contains(noLabel, ":") {
			noLabel = noLabel[strings.Index(noLabel, ":")+1:]
		}
		noLabel = strings.Trim(noLabel, " \t")

		if noLabel == "" {
			continue
		}

		//obtaining comma separated fields and the op code
		spaceFields := strings.Fields(noLabel)
		opCode := spaceFields[0]
		rest := strings.Join(spaceFields[1:], "")
		fields := strings.Split(rest, ",")

		if len(fields) == 0 {
			assemblyReportError(l, "opcodes must have at least one parameter; saw none")
		}

		var instruction uint32 = 0

		switch strings.ToLower(opCode) {
		case "add":
			regs, _ := extractRTypeInfo(fields, l, 3)
			instruction = formRInstruction(opADD, regs[1], regs[2], regs[0], 0, fnADD)
			break
		case "addi":
			regs, imm, _ := extractStandardITypeInfo(fields, l, labels, 0xFFFF0000, true)
			instruction = formIInstruction(opADDI, regs[0], regs[1], imm)
			break
		case "addu":
			regs, _ := extractRTypeInfo(fields, l, 3)
			instruction = formRInstruction(opADDU, regs[1], regs[2], regs[0], 0, fnADDU)
			break
		case "addiu":
			regs, imm, _ := extractStandardITypeInfo(fields, l, labels, 0xFFFF0000, true)
			instruction = formIInstruction(opADDIU, regs[0], regs[1], imm)
			break
		case "and":
			regs, _ := extractRTypeInfo(fields, l, 3)
			instruction = formRInstruction(opAND, regs[1], regs[2], regs[0], 0, fnAND)
			break
		case "andi":
			regs, imm, _ := extractStandardITypeInfo(fields, l, labels, 0xFFFF0000, false)
			instruction = formIInstruction(opANDI, regs[0], regs[1], imm)
			break
		case "beq":
			regs, imm, _ := extractStandardITypeInfo(fields, l, labels, 0xFFFC0000, false)
			instruction = formIInstruction(opBEQ, regs[0], regs[1], imm/4)
			break
		case "bne":
			regs, imm, _ := extractStandardITypeInfo(fields, l, labels, 0xFFFC0000, false)
			instruction = formIInstruction(opBNE, regs[0], regs[1], imm/4)
			break
		case "div":
			regs, _ := extractRTypeInfo(fields, l, 2)
			instruction = formRInstruction(opDIV, regs[0], regs[1], regs[0], 0, fnDIV)
			break
		case "divu":
			regs, _ := extractRTypeInfo(fields, l, 2)
			instruction = formRInstruction(opDIVU, regs[0], regs[1], regs[0], 0, fnDIVU)
			break
		case "jr":
			regs, _ := extractRTypeInfo(fields, l, 1)
			instruction = formRInstruction(opJR, regs[0], regs[2], regs[1], 0, fnJR)
			break
		case "mfhi":
			regs, _ := extractRTypeInfo(fields, l, 1)
			instruction = formRInstruction(opMFHI, regs[0], regs[1], regs[0], 0, fnMFHI)
			break
		case "mflo":
			regs, _ := extractRTypeInfo(fields, l, 1)
			instruction = formRInstruction(opMFLO, regs[0], regs[1], regs[0], 0, fnMFLO)
			break
		case "mult":
			regs, _ := extractRTypeInfo(fields, l, 2)
			instruction = formRInstruction(opMULT, regs[0], regs[1], regs[0], 0, fnMULT)
			break
		case "multu":
			regs, _ := extractRTypeInfo(fields, l, 2)
			instruction = formRInstruction(opMULTU, regs[0], regs[1], regs[0], 0, fnMULTU)
			break
		case "xor":
			regs, _ := extractRTypeInfo(fields, l, 3)
			instruction = formRInstruction(opXOR, regs[1], regs[2], regs[0], 0, fnXOR)
			break
		case "or":
			regs, _ := extractRTypeInfo(fields, l, 3)
			instruction = formRInstruction(opOR, regs[1], regs[2], regs[0], 0, fnOR)
			break
		case "ori":
			regs, imm, _ := extractStandardITypeInfo(fields, l, labels, 0xFFFF0000, false)
			instruction = formIInstruction(opORI, regs[0], regs[1], imm)
			break
		case "slt":
			regs, _ := extractRTypeInfo(fields, l, 3)
			instruction = formRInstruction(opSLT, regs[1], regs[2], regs[0], 0, fnSLT)
			break
		case "slti":
			regs, imm, _ := extractStandardITypeInfo(fields, l, labels, 0xFFFF0000, true)
			instruction = formIInstruction(opSLTI, regs[0], regs[1], imm)
			break
		case "sltiu":
			regs, imm, _ := extractStandardITypeInfo(fields, l, labels, 0xFFFF0000, false)
			instruction = formIInstruction(opSLTIU, regs[0], regs[1], imm)
			break
		case "sltu":
			regs, _ := extractRTypeInfo(fields, l, 3)
			instruction = formRInstruction(opSLTU, regs[1], regs[2], regs[0], 0, fnSLTU)
			break
		case "sll":
			regs, v, _ := extractStandardITypeInfo(fields, l, labels, 0xFFFF0000, false)
			if v > 31 {
				//invalid shift amount
				assemblyReportError(l, "cannot shift by more than 31 bits and cannot be a negative number")
				v = v & 0x1F //just to make it keep going
			}
			instruction = formRInstruction(opSLL, regs[1], 0, regs[0], int(v), fnSLL)
			break
		case "srl":
			regs, v, _ := extractStandardITypeInfo(fields, l, labels, 0xFFFF0000, false)
			if v > 31 {
				//invalid shift amount
				assemblyReportError(l, "cannot shift by more than 31 bits and cannot be a negative number")
				v = v & 0x1F //just to make it keep going
			}
			instruction = formRInstruction(opSRL, regs[1], 0, regs[0], int(v), fnSRL)
			break
		case "sra":
			regs, v, _ := extractStandardITypeInfo(fields, l, labels, 0xFFFF0000, false)
			if v > 31 {
				//invalid shift amount
				assemblyReportError(l, "cannot shift by more than 31 bits and cannot be a negative number")
				v = v & 0x1F //just to make it keep going
			}
			instruction = formRInstruction(opSRA, regs[1], 0, regs[0], int(v), fnSRA)
			break
		case "sllv":
			regs, _ := extractRTypeInfo(fields, l, 3)
			instruction = formRInstruction(opSLL, regs[1], regs[2], regs[0], 0, fnSLLV)
			break
		case "srlv":
			regs, _ := extractRTypeInfo(fields, l, 3)
			instruction = formRInstruction(opSRL, regs[1], regs[2], regs[0], 0, fnSRLV)
			break
		case "srav":
			regs, _ := extractRTypeInfo(fields, l, 3)
			instruction = formRInstruction(opSRA, regs[1], regs[2], regs[0], 0, fnSRAV)
			break
		case "sub":
			regs, _ := extractRTypeInfo(fields, l, 3)
			instruction = formRInstruction(opSUB, regs[1], regs[2], regs[0], 0, fnSUB)
			break
		case "subu":
			regs, _ := extractRTypeInfo(fields, l, 3)
			instruction = formRInstruction(opSUBU, regs[1], regs[2], regs[0], 0, fnSUBU)
			break
		case "lw":
			regs, v, _ := extractSpecialITypeInfo(fields, l, labels)
			instruction = formIInstruction(opLW, regs[0], regs[1], v)
			break
		case "lb":
			regs, v, _ := extractSpecialITypeInfo(fields, l, labels)
			instruction = formIInstruction(opLB, regs[0], regs[1], v)
			break
		case "lbu":
			regs, v, _ := extractSpecialITypeInfo(fields, l, labels)
			instruction = formIInstruction(opLBU, regs[0], regs[1], v)
			break
		case "sw":
			regs, v, _ := extractSpecialITypeInfo(fields, l, labels)
			instruction = formIInstruction(opSW, regs[0], regs[1], v)
			break
		case "sb":
			regs, v, _ := extractSpecialITypeInfo(fields, l, labels)
			instruction = formIInstruction(opSB, regs[0], regs[1], v)
			break
		case "j":
			v, e := getLiteralValue(fields[0], labels)
			if e != nil {
				assemblyReportError(l, e.Error())
			}
			instruction = formJInstruction(opJ, v/4)
			break
		case "jal":
			v, e := getLiteralValue(fields[0], labels)
			if e != nil {
				assemblyReportError(l, e.Error())
			}
			instruction = formJInstruction(opJAL, v/4)
			break
		case "swi":
			v, e := getLiteralValue(fields[0], labels)
			if e != nil {
				assemblyReportError(l, e.Error())
			}
			instruction = formIInstruction(opSWI, 0, 0, v)
			break
		case "lui":
			reg, v, _ := extractLUIInfo(fields, l, labels)
			instruction = formIInstruction(opLUI, reg, 0, v)
			break
		case "nop":
			instruction = 0
		default:
			assemblyReportError(l, "invalid opcode \""+opCode+"\". Note that this assembler only supports the"+
				" MIPS core ISA and does not support pseudo-opcodes")
		}

		insertMemoryValue(currentAddr, instruction, ret)
		lineRet[currentAddr] = l
		if opCode == "jal" {
			//adding NOP after JAL
			currentAddr += 4
			insertMemoryValue(currentAddr, 0, ret)
		}

		currentAddr += 4
	}

	return ret, lineRet
}

func Assemble(file string, settings AssemblySettings) (SystemMemory, map[uint32]InputLine, int, map[string]uint32) {
	//input will be newline delimited
	numErrors = 0
	lines := strings.Split(file, "\n")

	var textLines []InputLine
	var dataLines []InputLine

	//extracting the text and data lines from the code
	mode := assemExtractNone
	for i, l := range lines {

		//line preconditioning
		l = strings.Trim(l, " \t\r\n")
		l = strings.ReplaceAll(l, "\t", " ")

		//assembler directive detection
		if strings.Index(l, ".data ") == 0 || l == ".data" {
			mode = assemExtractData
			l = strings.Replace(l, ".data", "", 1)
			if l == "" {
				continue
			}
		} else if strings.Index(l, ".text") == 0 || l == ".text" {
			mode = assemExtractText
			l = strings.Replace(l, ".text", "", 1)
			if l == "" {
				continue
			}
		}

		//acting on directives
		if mode == assemExtractData {
			dataLines = append(dataLines, InputLine{
				Contents:   l,
				LineNumber: i + 1, //lines are 1 indexed
			})
		} else if mode == assemExtractText {
			textLines = append(textLines, InputLine{
				Contents:   l,
				LineNumber: i + 1,
			})
		}
	}

	dataMem, labels := assembleData(dataLines, settings)
	labels = extractTextLabels(textLines, settings, labels)
	textMem, lineRet := assembleText(textLines, settings, labels)

	//checking to ensure the data memory and text memory don't overlap
	if dataMem.startingAddr < textMem.startingAddr && dataMem.startingAddr+uint32(len(dataMem.memory)) >= textMem.startingAddr {
		//collision
		assemblyReportError(InputLine{
			Contents:   "{overall file}",
			LineNumber: 0,
		}, "assembled text and data memory overlaps, change the settings and assemble again")
		//no need to return now because it will be caught later
	} else if textMem.startingAddr < dataMem.startingAddr && textMem.startingAddr+uint32(len(textMem.memory)) >= dataMem.startingAddr {
		//collision
		assemblyReportError(InputLine{
			Contents:   "{overall file}",
			LineNumber: 0,
		}, "assembled text and data memory overlaps, change the settings and assemble again")
		//no need to return now because it will be caught later
	}

	//creating system memory
	sysMem := make(SystemMemory)
	sysMem = addToSystemMemory(textMem, sysMem)
	sysMem = addToSystemMemory(dataMem, sysMem)

	return sysMem, lineRet, numErrors, labels
}
