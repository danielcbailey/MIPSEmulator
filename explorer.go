package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

/**
 * The explorer is the command-line interface for interacting with the results after emulation has completed.
 * At this time, the explorer does not allow real-time debugging and only views the results of the emulation
 * including the final state of snap shots.
 *
 * Having said that, the vetting system captures select snapshots of failed tests, which may be useful.
 * The via the explorer, the following information is available about a given snapshot:
 *  - All register final values
 *  - All memory values
 *  - Label evaluations
 *  - Vet scenario
 *  - Specific runtime errors
 */

func startExplorer(latest EmulationResult, vSession *VetSession, labels map[string]uint32, lineMeta map[uint32]InputLine) {
	fmt.Println("\n+==== [ EXPLORER ]====+")
	fmt.Println("The explorer lets you explore failed cases or the last emulation.")
	fmt.Println("The current selection is the latest emulation, and does not necessarily mean it is a failed case.")
	numSnap := 1
	if vSession != nil {
		numSnap += len(vSession.FailedSnapshots)
	}
	fmt.Printf("Captured %d snapshots.\n", numSnap)
	fmt.Println("Type 'quit' to exit. Type 'help' for command assistance.")

	reader := bufio.NewReader(os.Stdin)
	selectionIndex := 0
	selection := &latest
	for true {
		fmt.Printf("R%d> ", selectionIndex)
		input, e := reader.ReadString('\n')
		if e != nil {
			fmt.Println("\nExiting...")
			return
		}

		input = strings.Trim(input, "\n \t\r")
		iLower := strings.ToLower(input)

		if iLower == "help" {
			displayHelp()
			continue
		} else if iLower == "quit" {
			fmt.Println("\nExiting...")
			return
		}

		fields := strings.Fields(iLower)
		oFields := strings.Fields(input)
		if len(fields) == 0 {
			continue
		}

		if fields[0] == "search" {
			//search command
			if vSession != nil {
				searchCommand(vSession.FailedSnapshots, fields)
			} else {
				searchCommand(nil, fields)
			}
		} else if fields[0] == "cr" {
			//change result command
			nSel := changeResultCommand(numSnap, oFields)
			if nSel == 0 {
				selectionIndex = 0
				selection = &latest
			} else if nSel != -1 && vSession != nil {
				selectionIndex = nSel
				selection = &vSession.FailedSnapshots[nSel-1].Snapshot
			}
		} else if fields[0] == "label" {
			//label decode command
			if len(oFields) != 2 {
				fmt.Println("[label] Invalid format, expected 'label myLabel'.")
				continue
			}

			res, e := getLiteralValue(oFields[1], labels)
			if e != nil {
				fmt.Println("[label] Invalid label:", e.Error())
				continue
			}
			fmt.Printf("[label] %s evaluates to %d (0x%X)\n", oFields[1], res, res)
		} else if fields[0] == "decode" {
			//address decode command
			if len(oFields) != 2 {
				fmt.Println("[decode] Invalid format, expected 'decode 0x1000'.")
				continue
			}

			res, e := getLiteralValue(oFields[1], labels)
			if e != nil {
				fmt.Println("[decode] Invalid address:", e.Error())
				continue
			}
			l, ok := lineMeta[res]
			if !ok {
				fmt.Printf("[decode] %s does not correspond to a line of assembly.\n", oFields[1])
			} else {
				fmt.Printf("[decode] %s corresponds to line %d \"%s\"\n", oFields[1], l.LineNumber, l.Contents)
			}
		} else if fields[0] == "scenario" {
			//scenario command
			displayScenario(selection)
		} else if fields[0] == "errors" {
			//errors display command
			errorsCommand(selection)
		} else if fields[0] == "saveimage" {
			genImageP1Fa21(selection)
		} else if fields[0] == "dump" {
			genFa21Project1Dump(selection)
		} else if len(fields[0]) > 0 && fields[0][0] == '$' {
			//register display
			displayRegisters(selection, input)
		} else if len(fields[0]) > 0 && fields[0][0] == '*' {
			//memory display
			displayMemory(selection, input, labels)
		}
	}
}

func displayHelp() {
	fmt.Println("+==== HELP ====+")
	fmt.Println("search [testcase] [optional: testcase 2]... | Returns list of failed result indices for the given test cases.")
	fmt.Println(" - These test cases should be from different categories.")
	fmt.Println(" - Example usage: 'search 1hLines ObsNone GeoL'")
	fmt.Println("cr [result index] | Changes current result to the specified index")
	fmt.Println(" - Note that index 0 is the last emulation result and is always available.")
	fmt.Println(" - Example usage: 'cr 2'")
	fmt.Println("$[register] | displays last register contents for the given register.")
	fmt.Println(" - Can be used in a range, for example: '$3 - 6' prints all register values in that range.")
	fmt.Println(" - Example usage: '$3'")
	fmt.Println("*[address] | displays last contents of that memory address")
	fmt.Println(" - Can be used in a range to print all contents within the range, example: '*0x400 - 0x40F'")
	fmt.Println(" - Addresses can be specified in hex, decimal, or label")
	fmt.Println(" - Example usage: '*5475'")
	fmt.Println("label [label name] | displays the value the label evaluates to.")
	fmt.Println(" - Example usage: 'label loopStart'")
	fmt.Println("decode [address] | displays the line of assembly that corresponds to that address")
	fmt.Println(" - Addresses can be specified in hex, decimal, or label")
	fmt.Println(" - Example usage: 'decode 0x4004'")
	fmt.Println("errors | displays all errors for the current result snapshot")
	fmt.Println(" - Example usage: 'errors'")
	fmt.Println("scenario | displays scenario information for the current snapshot")
	fmt.Println(" - Example usage: 'scenario'\n")
	fmt.Println("saveimage | saves the image of the current snapshot's test case")
	fmt.Println(" - Example usage: 'saveimage'")
	fmt.Println("dump | generates a dump file of the test case of the current snapshot that can be imported to MiSaSiM")
	fmt.Println(" - Example usage: 'dump'")
}

func errorsCommand(snap *EmulationResult) {
	if len(snap.Errors) == 0 {
		fmt.Println("[errors] This snapshot has no errors.\n")
		return
	}

	for _, e := range snap.Errors {
		fmt.Printf("[errors] %s; %s\n", decodeErrorCode(e.EType), e.Message)
	}

	fmt.Println()
}

func changeResultCommand(numSnap int, fields []string) int {
	if len(fields) != 2 {
		fmt.Println("[cr] invalid command usage, expected an index of the target snapshot. Use the search command to find indices.")
		return -1
	}

	n, e := strconv.Atoi(fields[1])
	if e != nil {
		fmt.Println("[cr] invalid index: the index is not an integer in base 10.")
		return -1
	}

	if n >= numSnap || n < 0 {
		fmt.Println("[cr] invalid index: the index does not exist.")
		return -1
	}

	return n
}

func searchCommand(snaps []VetSnapshot, fields []string) {
	results := make(map[int]string)

	for i := 1; len(fields) > i; i++ {
		for j := 0; len(snaps) > j; j++ {
			if strings.Contains(strings.ToLower(snaps[j].TestCase), strings.ToLower("-"+fields[i])) {
				//adding the result
				results[j] = snaps[j].TestCase
			}
		}
	}

	fmt.Printf("[search] Found %d results.\n", len(results))

	for i, s := range results {
		fmt.Printf("[search] Index %d: %s\n", i+1, s)
	}

	fmt.Println()
}

func displayMemory(snap *EmulationResult, input string, labels map[string]uint32) {
	input = strings.Trim(input, "*")
	if strings.Contains(input, "-") {
		//range
		r := strings.Split(input, "-")
		if len(r) != 2 {
			fmt.Println("[memory] Invalid range format. Expected '*0x100 - 0x110'")
			return
		}
		a1 := strings.Trim(r[0], " *")
		a2 := strings.Trim(r[1], " *")
		a1v, e := getLiteralValue(a1, labels)
		if e != nil {
			fmt.Printf("[memory] Invalid memory address: %s\n", e.Error())
			return
		}
		a2v, e := getLiteralValue(a2, labels)
		if e != nil {
			fmt.Printf("[memory] Invalid memory address: %s\n", e.Error())
			return
		}

		if a2v < a1v {
			fmt.Println("[memory] Invalid range. Must be 'smaller - larger'")
			return
		}

		for i := a1v; a2v >= i; i += 4 {
			if i-a1v > 100 {
				fmt.Printf("[memory] and %d more...\n", a2v-i)
				break
			}

			mv, ok := snap.Memory.memRead(i)
			if !ok {
				fmt.Printf("[memory] *0x%X = uninitialized\n", i)
				continue
			}

			fmt.Printf("[memory] *0x%X = %d (0x%X)\n", i, mv, mv)
		}
		fmt.Println()
	} else {
		//no range, just single address
		a, e := getLiteralValue(input, labels)
		if e != nil {
			fmt.Printf("[memory] Invalid memory address: %s\n", e.Error())
			return
		}

		mv, ok := snap.Memory.memRead(a)
		if !ok {
			fmt.Printf("[memory] *0x%X = uninitialized\n\n", a)
			return
		}

		fmt.Printf("[memory] *0x%X = %d (0x%X)\n\n", a, mv, mv)
	}
}

func displayRegisters(snap *EmulationResult, input string) {
	input = strings.Trim(input, "$")
	if strings.Contains(input, "-") {
		//range
		r := strings.Split(input, "-")
		if len(r) != 2 {
			fmt.Println("[registers] Invalid range format. Expected '$3 - 6'")
			return
		}
		a1 := strings.Trim(r[0], " $")
		a2 := strings.Trim(r[1], " $")
		a1v, e := getLiteralValue(a1, nil)
		if a1v > 31 {
			e = fmt.Errorf("registers are between 0 and 31")
		}
		if e != nil {
			fmt.Printf("[registers] Invalid register: %s\n", e.Error())
			return
		}
		a2v, e := getLiteralValue(a2, nil)
		if a2v > 31 {
			e = fmt.Errorf("registers are between 0 and 31")
		}
		if e != nil {
			fmt.Printf("[registers] Invalid register: %s\n", e.Error())
			return
		}

		if a2v < a1v {
			fmt.Println("[registers] Invalid range. Must be 'smaller - larger'")
			return
		}

		for i := a1v; a2v >= i; i += 1 {
			if i-a1v > 100 {
				fmt.Printf("[memory] and %d more...\n", a2v-i)
				break
			}

			mv, ok := snap.regRead(int(i))
			if !ok {
				fmt.Printf("[memory] $%d = uninitialized\n", i)
				continue
			}

			fmt.Printf("[memory] $%d = %d (0x%X)\n", i, mv, mv)
		}
		fmt.Println()
	} else {
		//no range, just single address
		a, e := getLiteralValue(input, nil)
		if a > 31 {
			e = fmt.Errorf("registers are between 0 and 31")
		}
		if e != nil {
			fmt.Printf("[registers] Invalid register: %s\n", e.Error())
			return
		}

		mv, ok := snap.regRead(int(a))
		if !ok {
			fmt.Printf("[registers] $%d = uninitialized\n\n", a)
			return
		}

		fmt.Printf("[registers] $%d = %d (0x%X)\n\n", a, mv, mv)
	}
}

func displayScenario(selection *EmulationResult) {
	scen, _ := json.Marshal(selection.SWIContext)
	fmt.Println("[scenario]", string(scen), "\n")
}
