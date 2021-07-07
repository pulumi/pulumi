import { RandomPassword } from "@pulumi/random";
import { PolicyPack, validateResourceOfType } from "@pulumi/policy";

new PolicyPack("random", {
    policies: [{
        name: "password-minimum-length",
        description: "Ensures that the password length is 10 or more characters",
        enforcementLevel: "mandatory",
        validateResource: validateResourceOfType(RandomPassword, (password, _, reportViolation) => {
            if (password.length < 10) {
                reportViolation(
                    "Password must be 10 characters or more");
            }
        }),
    }],
});
