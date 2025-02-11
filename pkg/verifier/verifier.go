package verifier

import (
	"github.com/tonkeeper/tongo/ton"
)

type SourceCompiler string

const (
	SourceCompilerFunc SourceCompiler = "func"
	SourceCompilerFift SourceCompiler = "fift"
	SourceCompilerTact SourceCompiler = "tact"
)

type File struct {
	Name             string `json:"name"`
	Content          string `json:"content"`
	IsEntrypoint     bool   `json:"is_entrypoint"`
	IsStdLib         bool   `json:"is_std_lib"`
	IncludeInCommand bool   `json:"include_in_command"`
}

type Source struct {
	Code             string         `json:"code"`
	DisassembleCode  string         `json:"disassemble_code"`
	Files            []File         `json:"files"`
	Compiler         SourceCompiler `json:"compiler"`
	DateVerification int64          `json:"date_verification"`
}

type Verifier struct{}

func NewVerifier() *Verifier {
	return &Verifier{}
}

func (v *Verifier) GetAccountSource(accountID ton.AccountID) (Source, error) {
	return Source{}, nil
}
