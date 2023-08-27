package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"visualization/script"
	_type "visualization/type"
)

func main() {
	if !decodeArgs(os.Args) {
		fillDefault()
	}

	defer release()

	//http.HandleFunc("/", rootHandler)
	http.HandleFunc("/span_info", script.VisSpanInfoHandler)
	//http.HandleFunc("/option2", option2Handler)

	fmt.Printf("Server started at :%s\n", _type.DstPort)
	if err := http.ListenAndServe(":"+_type.DstPort, nil); err != nil {
		fmt.Println(err.Error())
	}
}

func release() {

}

func fillDefault() {
	_type.DstPort = "11235"
	_type.SrcPort = "6001"
	_type.SrcHost = "127.0.0.1"
	_type.SourceFile = ""
}

// type 1: -http=:dstPort -hSrcHost -PSrcPort -uSrcUsrName -pSrcPwd
// type 2: -f srcFile
const (
	ArgsFormat1 = 3
	ArgsFormat2 = 6
)

func decodeArgs(args []string) bool {
	if len(args) == ArgsFormat1 {
		if args[1] != "-f" {
			return false
		}
		_type.SourceFile = args[2]
		return true
	} else if len(args) == ArgsFormat2 {
		idx := map[string]*string{
			"-http=:": &_type.DstPort,
			"-h":      &_type.SrcHost,
			"-p":      &_type.SrcPassword,
			"-u":      &_type.SrcUsrName,
			"-P":      &_type.SrcPort,
		}

		for p, o := range idx {
			curArg := ""
			for _, arg := range args {
				if strings.HasPrefix(arg, p) {
					curArg = arg
				}
			}
			if curArg == "" {
				return false
			}
			*o = strings.Trim(curArg, p)
		}

		return true
	}

	return false
}
