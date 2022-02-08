package utils

import (
	"encoding/json"
	"github.com/thoas/go-funk"
)


func ToJsonLines (nodes []map[string]interface{}) (jsonLines []byte,err  error){
	funk.ForEach(nodes, func(node map[string]interface{}){
		nodeBytes, err  := json.Marshal(node)
		if err != nil{
			return
		}
		nodeBytes = append(nodeBytes,  []byte{'\n'}...)
		jsonLines = append(jsonLines, nodeBytes...)
	})
	return jsonLines, nil
}
