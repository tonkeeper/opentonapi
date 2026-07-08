package references

type ExtendedCode int64

const (
	ErrGaslessJettonIsNotSupported ExtendedCode = iota + 40_000
	ErrGaslessTemporary
	ErrGaslessSignature
	ErrGaslessPendingMessages
	ErrGaslessBadRequest
	ErrGaslessOperationIsNotSupported
	ErrGaslessUserDisabled
	ErrGaslessEstimatingCommission
	ErrGaslessCommission
	ErrGaslessBalance
	ErrGaslessBootstrapTransferDisabled
	ErrGaslessUnknown
	ErrGaslessNotEnoughJettons
	ErrGaslessUnsupportedExtension
)

const (
	// ErrInsufficientTONForGas is reported when the source wallet lacks TON to cover transfer gas.
	// The error carries the required/available amounts in Error.insufficient_funds.
	ErrInsufficientTONForGas ExtendedCode = iota + 50_000
)
