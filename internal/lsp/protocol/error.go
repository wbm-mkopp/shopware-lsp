package protocol

type ShopwareLspError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewLspError(message string, code string) *ShopwareLspError {
	return &ShopwareLspError{
		Code:    code,
		Message: message,
	}
}
