package com.acme.hello;

@javax.annotation.Generated(value = "jsii-pacmak/0.13.3 (build 3624e0f)", date = "2019-07-08T18:24:28.241Z")
public interface IOutput extends software.amazon.jsii.JsiiSerializable {
    java.lang.Object val();

    /**
     * A proxy class which represents a concrete javascript instance of this type.
     */
    final static class Jsii$Proxy extends software.amazon.jsii.JsiiObject implements com.acme.hello.IOutput {
        protected Jsii$Proxy(final software.amazon.jsii.JsiiObject.InitializationMode mode) {
            super(mode);
        }

        @Override
        @javax.annotation.Nullable
        public java.lang.Object val() {
            return this.jsiiCall("val", java.lang.Object.class);
        }
    }
}
