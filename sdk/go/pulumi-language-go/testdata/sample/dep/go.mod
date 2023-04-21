module example.com/dep

go 1.18

replace example.com/indirect-dep/v2 => ../indirect-dep

require example.com/indirect-dep/v2 v2.1.0
