import * as pulumi from "@pulumi/pulumi";

import { Echo } from "./echo";

export = async () => {
  const b = new pulumi.StackReference("organization/multistack-b/dev")

  const echo = new Echo("echo", { echo: b.getOutput("output") });

  return {
    output: echo.echo,
  };
}
