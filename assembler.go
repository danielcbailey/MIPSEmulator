package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

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
		//expanding memory
		mem.memory = append(mem.memory, 0)
	}

	//inserting the value
	prev := mem.memory[addr/4]
	prev = prev | (value << ((addr % 4) * 8))
	mem.memory[addr/4] = prev
}

func getLiteralValue(s string, labels map[string]uint32) (uint32, error) {
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
					ret = (ret << 4) | uint32(c-'a')
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
		return v, nil
	}
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

		switch fields[1] {
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

		if noLabel != "" {
			currentAddr += 4
		} else {
			if strings.Contains(noComment, ":") {
				//label on an empty line, not allowed
				assemblyReportError(l, "cannot declare labels on lines without assembly operations")
			}
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
	}

	return labels
}

func assembleText(lines []InputLine, settings AssemblySettings) *MemoryImage {

}

func assemble(file string, settings AssemblySettings) (*MemoryImage, map[uint32]InputLine) {
	retMem := new(MemoryImage)
	retMap := make(map[uint32]InputLine)

	//input will be newline delimited
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

}
