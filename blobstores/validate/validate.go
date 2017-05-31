package validate

func NotEmptyMessage(s string, panicMessage string) {
	if s == "" {
		panic(panicMessage)
	}
}

func NotEmpty(s string) {
	if s == "" {
		panic("String must not be empty")
	}
}
