import pulumi
import pulumi_docs as docs

res = docs.Resource("res", in_=docs.fun_output(in_=False).apply(lambda invoke: invoke.out))
