resource "source" "partial:index:Source" {
}

resource "consumer" "partial:index:Consumer" {
    values = {
        "dataKnownField" = source.data.knownField
        "dataKnownSecretField" = source.data.knownSecretField
        "dataUnknownField" = source.data.unknownField
        "dataUnknownSecretField" = source.data.unknownSecretField
        "listKnown" = source.dataList[0]
        "listKnownSecret" = source.dataList[1]
        "listUnknown" = source.dataList[2]
        "listUnknownSecret" = source.dataList[3]
        "mapKnown" = source.dataMap["knownKey"]
        "mapKnownSecret" = source.dataMap["knownSecretKey"]
        "mapUnknown" = source.dataMap["unknownKey"]
        "mapUnknownSecret" = source.dataMap["unknownSecretKey"]
    }
}
