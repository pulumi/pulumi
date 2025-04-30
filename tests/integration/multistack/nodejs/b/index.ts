import * as pulumi from "@pulumi/pulumi";

import { Echo } from "./echo";

export = async () => {
  const echo = new Echo("echo", { echo: "hello world" });

  return {
    output: echo.echo,
  };
}
