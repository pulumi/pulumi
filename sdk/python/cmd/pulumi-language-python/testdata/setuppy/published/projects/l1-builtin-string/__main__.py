import pulumi
import unicodedata

def grapheme_length(s):
    count, prev_zwj = 0, False
    for c in s:
        if prev_zwj:
            prev_zwj = False
            continue
        if c == '\u200d':
            prev_zwj = True
            continue
        if unicodedata.category(c)[0] != 'M':
            count += 1
    return count


config = pulumi.Config()
a_string = config.require("aString")
pulumi.export("lengthOutput", grapheme_length(a_string))
pulumi.export("splitOutput", a_string.split("-"))
pulumi.export("joinOutput", "|".join(a_string.split("-")))
pulumi.export("interpolateOutput", f"prefix-{a_string}")
