import * as pulumi from "@pulumi/pulumi";
import * as random from "@pulumi/random";

const numbers: random.RandomInteger[] = [];
for (const range = {value: 0}; range.value < 2; range.value++) {
    numbers.push(new random.RandomInteger(`numbers-${range.value}`, {
        min: 1,
        max: range.value,
        seed: `seed${range.value}`,
    }));
}
export const first = numbers[0].id;
export const second = numbers[1].id;
