# MIPSEmulator / Vetter for Gatech ECE 2035

## Perform 100,000 tests on your MIPS assembly and analyze the result and failed cases

Statistically, in one vet session it will check *every* possible reference for project one 15 times.

## Usage

A precompiled executable has been included for convenience for those on Windows 64 bit and allows you to avoid compiling the program.

To compile the source, one must have Golang installed (only required to build the binary, not to use it). Instructions for how to install and build with Go can be found in the section [Compilation](https://github.com/danielcbailey/MIPSEmulator/tree/main#compilation)

To use the program, open it from a terminal or by opening the executable file. The program is a command-line interface program.
Then, a wizard will walk through the settings. These settings are:
1. Assembly file (relative or absolute path)
2. Number of errors to tolerate before halting a sample
3. The assignment to use for the vet process (can be left blank to disable vetting; leaving blank will only emulate one sample)

It will then automatically assemble the assembly file and if any errors are generated, they will be displayed and the program will end.
In order to proceed to emulation, the assembly must not generate any errors, and the assembler is strict.

If your program generates too many infinite loops, the batch emulation will stop and will report on only the samples processed up until that point.

Following emulation, basic info such as average DI will be displayed, then vetting info if it is enabled.

At the end of any emulation, batch or single, the program will launch into the explorer which allows for post-run analysis of snapshots.
To conserve on memory, only some snapshots are captured of all eligible ones. It will always capture the last emulation, and it will randomly\* select failed snapshots to save for the explorer.
\* The random probability of capture exponentially decreases with the number of similar test-case snapshots captured.

To learn more about the explorer and its specific features, type "help" into the explorer command line when the program launches it after an emulation.

## Compilation

If you do not already have Golang installed:

1. [Download Go](https://golang.org/dl/) and follow the instructions on the installer
2. Verify the installation by running `go version` in a terminal

Compiling the program

1. Download a copy of the source code (simplist way is to download the zip)
2. **Important:** Navigate to your Go `src` folder. It is typically located in your user directory under `go`. For example for windows: `C:\users\aUser\go\src`
3. Create a folder in the `src` directory. It can be named anything. Copy the source code into that directory.
4. From a terminal, navigate to the directory you just put the source code in.
5. From that terminal, run `go build -o MIPSVet.exe main.go emulator.go explorer.go softwareInterrupts.go analysis.go project1.go assembler.go instructions.go eula.go`
