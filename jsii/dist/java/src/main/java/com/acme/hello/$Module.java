package com.acme.hello;

import software.amazon.jsii.JsiiModule;

public final class $Module extends JsiiModule {
    public $Module() {
        super("experiments", "1.0.0", $Module.class, "experiments@1.0.0.jsii.tgz");
    }

    @Override
    protected Class<?> resolveClass(final String fqn) throws ClassNotFoundException {
        switch (fqn) {
            case "experiments.HelloJsii": return com.acme.hello.HelloJsii.class;
            case "experiments.IOutput": return com.acme.hello.IOutput.class;
            default: throw new ClassNotFoundException("Unknown JSII type: " + fqn);
        }
    }
}
