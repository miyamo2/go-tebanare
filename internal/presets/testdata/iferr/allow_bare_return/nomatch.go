package p

func f() (err error) {
	if err != nil {
		log.Print(err)
		return
	}
	if err := load(); err != nil {
		return
	}
	if err != nil { // the caller logs it
		return
	}
}
