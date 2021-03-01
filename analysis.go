package main

import "fmt"

type VetTestCase struct {
	Successes       int
	Fails           int
	ErrorsFrequency map[int]int //the key is the error type of the runtime error, the value is the amount of that type
	TotalErrors     int
}

type VetSession struct {
	Assignment   string
	CorrectCount int
	TotalCount   int
	TestCases    map[string]*VetTestCase
}

func addVetErrors(errors []RuntimeError, vErrors map[int]int) map[int]int {
	for _, e := range errors {
		v, ok := vErrors[e.EType]
		if !ok {
			vErrors[e.EType] = 1
		} else {
			vErrors[e.EType] = v + 1
		}
	}

	return vErrors
}

func newVet(aName string) *VetSession {
	ret := new(VetSession)
	ret.TestCases = make(map[string]*VetTestCase)
	ret.Assignment = aName
	return ret
}

func (v *VetSession) displayResults() {
	avgErr := 0.0
	for _, val := range v.TestCases {
		avgErr += float64(val.TotalErrors)
	}
	avgErr /= float64(v.TotalCount)

	fmt.Println("\n+====[ VET RESULTS ]====+")
	fmt.Printf("Vet for %s.\n", v.Assignment)
	fmt.Printf("Summary:\n")
	fmt.Printf(" - Performed %d tests.\n", v.TotalCount)
	fmt.Printf(" - Of those, %d were successful (%.3f%% success rate).\n", v.CorrectCount, float64(v.CorrectCount)/float64(v.TotalCount)*100)
	fmt.Printf(" - For each evaluation, on average there were %.3f errors.\n", avgErr)

	fmt.Printf("\nTest Cases (%d):\n", len(v.TestCases))
	for k, v := range v.TestCases {
		fmt.Printf(" - %s: Successes: %d; Fails: %d; Error Count: %d\n", k, v.Successes, v.Fails, v.TotalErrors)
		for ek, ef := range v.ErrorsFrequency {
			fmt.Printf("   + Error: %s; Count: %d (%.3f%%)\n", decodeErrorCode(ek), ef, float64(ef)/float64(v.TotalErrors)*100)
		}
	}
}

func displayGeneralResults(n, dimin, dimax, si int, avgdi float64, errors []RuntimeError, fName string) {
	fmt.Println("\n+====[ EMULATION RESULTS ]====+")
	fmt.Printf("Emulation of %s.\n", fName)
	fmt.Printf("Summary:\n")
	fmt.Printf(" - Performed %d tests.\n", n)
	fmt.Printf(" - %d SI; %5.2f average DI (min: %d, max: %d)\n", si, avgdi, dimin, dimax)

	if errors != nil {
		fmt.Printf(" - Total errors generated: %d\n", len(errors))
		fmt.Printf("\nAll errors:\n")
		for _, e := range errors {
			fmt.Printf(" - %s; %s\n", decodeErrorCode(e.EType), e.Message)
		}
	}
}
