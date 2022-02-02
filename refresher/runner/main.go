package main

import (
	"context"
	"fmt"
	"github.com/infralight/pulumi/refresher"
)
func main()  {
	var c = refresher.NewClient(context.Background(), "https://api.pulumi.com")

	//Login
	var b, err = c.Login()
	if err != nil{
		fmt.Errorf("could not connect to pulumi. error=%w", err)
	}

	stacks, token, err := c.ListStacks(b)
	if err != nil{
		fmt.Errorf("could not get list stacks. error=%w", err)
	}

	fmt.Println(fmt.Sprintf("--DEBUG-- Token %s", token))

	//Something
	c.TempRunner(b, stacks)

}

