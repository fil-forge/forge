package debug

type EchoArguments struct {
	Message string `cborgen:"message" dagjsongen:"message"`
}
