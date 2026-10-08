package domain

import "errors"

// ErrPrintJobAlreadyExists means the printer job was recorded by another client.
var ErrPrintJobAlreadyExists = errors.New("printer print job already exists")
