package main

import (
	"fmt"
	"github.com/hanzozt/sdk-golang/ziti/sdkinfo"
)

func main() {
	_, sdkInfo := sdkinfo.GetSdkInfo()
	fmt.Printf("%s", sdkInfo.Version)
}
