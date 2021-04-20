// Trampolines to math/big assembly implementations.

// +build mips mipsle

#include "textflag.h"

// func addVV(z, x, y []Word) (c Word)
TEXT ·addVV(SB),NOSPLIT,$0
	JMP	math∕big·addVV(SB)

// func subVV(z, x, y []Word) (c Word)
// (same as addVV except for SBBQ instead of ADCQ and label names)
TEXT ·subVV(SB),NOSPLIT,$0
	JMP	math∕big·subVV(SB)

// func addVW(z, x []Word, y Word) (c Word)
TEXT ·addVW(SB),NOSPLIT,$0
	JMP	math∕big·addVW(SB)

// func subVW(z, x []Word, y Word) (c Word)
// (same as addVW except for SUBQ/SBBQ instead of ADDQ/ADCQ and label names)
TEXT ·subVW(SB),NOSPLIT,$0
	JMP	math∕big·subVW(SB)

// func shlVU(z, x []Word, s uint) (c Word)
TEXT ·shlVU(SB),NOSPLIT,$0
	JMP	math∕big·shlVU(SB)

// func shrVU(z, x []Word, s uint) (c Word)
TEXT ·shrVU(SB),NOSPLIT,$0
	JMP	math∕big·shrVU(SB)

// func mulAddVWW(z, x []Word, y, r Word) (c Word)
TEXT ·mulAddVWW(SB),NOSPLIT,$0
	JMP	math∕big·mulAddVWW(SB)

// func addMulVVW(z, x []Word, y Word) (c Word)
TEXT ·addMulVVW(SB),NOSPLIT,$0
	JMP	math∕big·addMulVVW(SB)

