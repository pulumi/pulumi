resource bar "azure:costmanagement/v20191101:View" {
    scope = "subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/MYDEVTESTRG"
    viewName = "swaggerExample"
    eTag = "\"1d4ff9fe66f1d10\""
    displayName = "swagger Example"
    query = {
        type = "Usage"
        timeframe = "MonthToDate"
        dataset = {
            granularity = "Daily"
            aggregation = {
                totalCost = {
                    name = "PreTaxCost"
                    function = "Sum"
                }
            }
            grouping = []
            sorting = [
                {
                    direction = "Ascending"
                    name = "UsageDate"
                }
            ]
        }
    }
    chart = "Table"
    accumulated = true
    metric = "ActualCost"
    kpis = [
        {
            type = "Forecast"
            id = null
            enabled = true
        }
    ]
    pivots = [
        {
            type = "Dimension"
            name = "ServiceName"
        }
    ]
}
