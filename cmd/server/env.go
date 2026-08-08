package main

import "os"

func init() {
	osLookupEnv = os.LookupEnv
}