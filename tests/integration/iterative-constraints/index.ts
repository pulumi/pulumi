import * as pulumi from "@pulumi/pulumi";
import * as random from "@pulumi/random";

const count = new random.RandomInteger("count", { min: 1, max: 3 });

export const pets = count.result.apply((n) => {
  const result: random.RandomPet[] = [];
  for (let i = 0; i < n; i++) {
    result.push(new random.RandomPet(`petResource-${i}`));
  }
  return result;
});
