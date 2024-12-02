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
)
