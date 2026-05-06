package main

import (
)

type AdminConfig struct {
	Type        uint32 `json:"type"`
	SubType     uint32 `json:"subType"`
	Parent      string `json:"parent"`
	Name        string `json:"name"`
	ServiceName string
}
