package vm_indent

import (
	"fmt"

	"github.com/goccy/go-json/internal/encoder"
)

func DebugRun(ctx *encoder.RuntimeContext, b []byte, codeSet *encoder.OpcodeSet) ([]byte, error) {
	var code *encoder.Opcode
	if (ctx.Option.Flag & encoder.HTMLEscapeOption) != 0 {
		code = codeSet.EscapeKeyCode
	} else {
		code = codeSet.NoescapeKeyCode
	}

	defer func() {
		if err := recover(); err != nil {
			fmt.Println("=============[DEBUG]===============")
			fmt.Println("* [TYPE]")
			fmt.Println(codeSet.Type)
			fmt.Printf("\n")
			fmt.Println("* [ALL OPCODE]")
			fmt.Println(code.Dump())
			fmt.Printf("\n")
			fmt.Println("* [CONTEXT]")
			fmt.Printf("%+v\n", ctx)
			fmt.Println("===================================")
			panic(err)
		}
	}()

	return Run(ctx, b, codeSet)
}
