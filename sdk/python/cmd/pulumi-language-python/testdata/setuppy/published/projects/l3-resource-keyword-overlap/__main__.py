import pulumi
from keywordComponent import KeywordComponent

comp = KeywordComponent("comp", {
    'input': True})
pulumi.export("result", comp.result)
