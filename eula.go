package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

func generateEula(reader *bufio.Reader) {
	builder := strings.Builder{}

	builder.WriteString("MIPSVet Eula\n")
	builder.WriteString("By agreeing to this EULA, you agree to the following:\n")
	builder.WriteString(" + The software is provided as is and issues may be raised,\n" +
		" but no assistance may be given in resolving anything the software raises. This includes,\n" +
		" but is not limited to, debugging assembly and explaining why assembly generates a certain \n" +
		"error.\n")
	builder.WriteString(" + You, the user, are responsible for ensuring that your code complies with\n" +
		" the official assignment specification and that it works to a satisfactory degree using officially\n" +
		" provided course materials such as MiSaSiM.")
	builder.WriteString(" + The software author bares no responsibility for consequences of usage of the\n" +
		" software.\n")
	builder.WriteString(" + You may redistribute the software and use in any way possible, but the author\n" +
		" of this software is not responsible for what you do with the software or the source code.\n")
	builder.WriteString("eula=false")

	e := ioutil.WriteFile("eula.txt", []byte(builder.String()), 0644)
	if e != nil {
		fmt.Println("Error generating eula file:", e.Error())
		time.Sleep(4 * time.Second)
		os.Exit(3)
	}

	fmt.Println("+===[ IMPORTANT ]===+")
	fmt.Println("An EULA (End User Licence Agreement) has been generated in the directory of the executable.")
	fmt.Println("To use the software, please agree to the EULA. For convenience, the EULA is repeated here:\n")
	fmt.Println(builder.String())
	fmt.Println("\nTo agree to the EULA, either edit the file and restart the program, or type 'I agree' below.")
	statement, _ := reader.ReadString('\n')
	statement = strings.Trim(statement, " \n\t\r")
	if strings.ToLower(statement) != "i agree" {
		fmt.Println("Invalid agreement, please edit the file or restart this program.")
		time.Sleep(4 * time.Second)
		os.Exit(3)
	}

	fContents := builder.String()
	fContents = strings.Replace(fContents, "eula=false", "eula=true", 1)

	e = ioutil.WriteFile("eula.txt", []byte(fContents), 0644)
	if e != nil {
		fmt.Println("Error updating eula file:", e.Error())
		time.Sleep(4 * time.Second)
		os.Exit(3)
	}
}

func validateEula(reader *bufio.Reader) {
	_, e := os.Open("eula.txt")
	if e != nil {
		if os.IsNotExist(e) {
			generateEula(reader)
			return
		}
		fmt.Println("Error reading eula file:", e.Error())
		time.Sleep(4 * time.Second)
		os.Exit(3)
	}

	fContentsB, e := ioutil.ReadFile("eula.txt")
	if e != nil {
		fmt.Println("Error reading eula file:", e.Error())
		time.Sleep(4 * time.Second)
		os.Exit(3)
	}

	fContents := string(fContentsB)

	if strings.Contains(fContents, "eula=true") {
		//eula validated
		return
	} else if !strings.Contains(fContents, "eula=false") {
		//invalid eula
		generateEula(reader)
		return
	}

	fmt.Println("\nTo agree to the EULA, either edit the file and restart the program, or type 'I agree' below.")
	statement, _ := reader.ReadString('\n')
	statement = strings.Trim(statement, " \n\t\r")
	if strings.ToLower(statement) != "i agree" {
		fmt.Println("Invalid agreement, please edit the file or restart this program.")
		time.Sleep(4 * time.Second)
		os.Exit(3)
	}

	fContents = strings.Replace(fContents, "eula=false", "eula=true", 1)

	e = ioutil.WriteFile("eula.txt", []byte(fContents), 0644)
	if e != nil {
		fmt.Println("Error updating eula file:", e.Error())
		time.Sleep(4 * time.Second)
		os.Exit(3)
	}
}
