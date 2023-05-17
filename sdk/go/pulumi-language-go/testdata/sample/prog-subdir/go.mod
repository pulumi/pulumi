module example.com/prog-subdir

go 1.18

replace (
	example.com/dep => ../dep
	example.com/indirect-dep/v2 => ../indirect-dep
	example.com/plugin => ../plugin
)

require (
	example.com/dep v1.5.0
	example.com/plugin v1.2.3
)

require example.com/indirect-dep/v2 v2.1.0 // indirect
