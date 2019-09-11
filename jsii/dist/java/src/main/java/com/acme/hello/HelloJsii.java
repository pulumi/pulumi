package com.acme.hello;

@javax.annotation.Generated(value = "jsii-pacmak/0.13.3 (build 3624e0f)", date = "2019-07-08T18:24:28.238Z")
@software.amazon.jsii.Jsii(module = com.acme.hello.$Module.class, fqn = "experiments.HelloJsii")
public class HelloJsii extends software.amazon.jsii.JsiiObject {
    protected HelloJsii(final software.amazon.jsii.JsiiObject.InitializationMode mode) {
        super(mode);
    }
    public HelloJsii() {
        super(software.amazon.jsii.JsiiObject.InitializationMode.Jsii);
        software.amazon.jsii.JsiiEngine.getInstance().createNewObject(this);
    }

    public java.lang.Object baz(final java.lang.Object input) {
        return this.jsiiCall("baz", java.lang.Object.class, new Object[] { java.util.Objects.requireNonNull(input, "input is required") });
    }

    public java.util.List<java.lang.String> getStrings() {
        return this.jsiiGet("strings", java.util.List.class);
    }

    public void setStrings(final java.util.List<java.lang.String> value) {
        this.jsiiSet("strings", java.util.Objects.requireNonNull(value, "strings is required"));
    }
}
