package main

import (
	"fmt"
	"github.com/hanzozt/sdk-golang/zt/sdkinfo"
)

func main() {
	_, sdkInfo := sdkinfo.GetSdkInfo()
	fmt.Printf("%s", sdkInfo.Version)
}
