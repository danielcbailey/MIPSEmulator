package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"
)

/**
 * The entry point for the executable.
 * Admittedly, this file is poorly written but it is because it is specific to the executable and doesn't serve
 * much use outside of this context. By contrast, the assembler and emulator are intended to be repurpose-able.
 */

func main() {
	//wizard instead of arguments for now
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Assembly file:")
	asmFile, _ := reader.ReadString('\n')
	asmFile = strings.Trim(asmFile, " \n\t\r")

	b, e := ioutil.ReadFile(asmFile)
	if e != nil {
		fmt.Println("ERROR: Failed to open assembly file: " + e.Error())
		fmt.Println("Press enter to exit..")
		_, _ = reader.ReadByte()
		return
	}

	fmt.Println("Number of errors to tolerate per sample (blank will default to 5)")
	numETol, _ := reader.ReadString('\n')
	numETol = strings.Trim(numETol, " \n\t\r")
	eTol := 5
	if len(numETol) > 0 {
		eTol, e = strconv.Atoi(numETol)
		if e != nil {
			fmt.Println("Invalid number, defaulting to 5. Error:", e.Error())
			eTol = 5
		}
	}

	fmt.Println("Type the assignment to vet the assembly for. Leave blank for no vetting.")
	fmt.Println("Options are: 'P1' for Project 1")
	vetReq, _ := reader.ReadString('\n')
	vetReq = strings.Trim(vetReq, " \n\t\r")
	numSamples := 1
	var vetSession *VetSession
	if len(vetReq) > 0 {
		numSamples = 100000
		switch strings.ToLower(vetReq) {
		case "p1":
			vetSession = newVet("Project 1")
			break
		default:
			fmt.Println("unknown assignment to vet, continuing with no vet in 3 seconds")
			time.Sleep(3 * time.Second)
			numSamples = 1
		}
	}

	settings := AssemblySettings{
		TextStart: 0x1000,
		DataStart: 0x8000,
	}

	sysMem, lineMeta, numE, labels := Assemble(string(b), settings)
	if numE != 0 {
		fmt.Printf("%d error(s) generated from assembler, not attempting emulation.\n", numE)
		fmt.Println("Press enter to exit..")
		_, _ = reader.ReadByte()
		return
	}

	limit := 100000

	var lastResult EmulationResult
	numInf := 0
	dimin := limit
	dimax := 0
	avgDI := 0.0
	var sysMemCopy SystemMemory
	for i := 0; numSamples > i; i++ {
		//creating a copy of the memory
		sysMemCopy = make(SystemMemory)
		for k, v := range sysMem {
			newPage := MemoryPage{
				startAddr:   v.startAddr,
				memory:      make([]uint32, len(v.memory)),
				initialized: make([]uint32, len(v.initialized)),
			}

			copy(newPage.memory, v.memory)
			copy(newPage.initialized, v.initialized)

			sysMemCopy[k] = newPage
		}

		//performing the emulation
		lastResult = Emulate(settings.TextStart, sysMemCopy, uint32(limit), eTol)

		avgDI += float64(lastResult.DI)
		if int(lastResult.DI) < dimin {
			dimin = int(lastResult.DI)
		}
		if int(lastResult.DI) > dimax {
			dimax = int(lastResult.DI)
		}

		//checking health of output
		if len(lastResult.Errors) > 0 && lastResult.Errors[len(lastResult.Errors)-1].EType == eRuntimeLimitExceeded {
			numInf++

			if numInf > 10 {
				//too many infinite loops
				fmt.Println("\n+====[ HALTED DUE TO TOO MANY INFINITE LOOPS ]===+")
				numSamples = i + 1
				break
			}
		}

		if vetSession != nil {
			vetSession.vetP1Interop(lastResult)
		}

		//updating user every 10%
		if numSamples > 10000 && i%(numSamples/10) == 0 {
			fmt.Printf("Progress: Completed %d%% (%d emulations)\n", i/(numSamples/100), i)
		}
	}
	eSlice := lastResult.Errors
	if numSamples > 1 {
		eSlice = nil
	}

	displayGeneralResults(numSamples, dimin, dimax, len(lineMeta), avgDI/float64(numSamples), eSlice, asmFile)

	if vetSession != nil {
		vetSession.displayResults()
	}

	startExplorer(lastResult, vetSession, labels, lineMeta)
}
