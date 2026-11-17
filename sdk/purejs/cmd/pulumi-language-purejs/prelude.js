// Host prelude for purejs policy packs, evaluated before the pack's bundle. It provides the few Node.js globals that
// policies use, and the entry points the host calls.
// Go provides: __log(level, msg), __b64dec(s), __b64enc(s), __parseURL(input, base).
(function (g) {
    "use strict";

    function fmt(args) {
        return Array.prototype.map.call(args, function (a) {
            if (typeof a === "string") return a;
            try { return JSON.stringify(a); } catch (e) { return String(a); }
        }).join(" ");
    }
    g.console = {
        log: function () { __log("info", fmt(arguments)); },
        info: function () { __log("info", fmt(arguments)); },
        debug: function () { __log("debug", fmt(arguments)); },
        warn: function () { __log("warn", fmt(arguments)); },
        error: function () { __log("error", fmt(arguments)); },
    };

    // Minimal Buffer: only what policies use (decode base64/utf8 text and read it back as a string).
    function Buffer(s) { this._s = s; this.length = s.length; }
    Buffer.from = function (input, enc) {
        if (input instanceof Buffer) return new Buffer(input._s);
        if (typeof input !== "string") input = String(input);
        if (enc === "base64" || enc === "base64url") return new Buffer(__b64dec(input));
        return new Buffer(input);
    };
    Buffer.isBuffer = function (b) { return b instanceof Buffer; };
    Buffer.byteLength = function (s) { return s.length; };
    Buffer.prototype.toString = function (enc) {
        if (enc === "base64") return __b64enc(this._s);
        return this._s;
    };
    g.Buffer = Buffer;
    g.atob = function (s) { return __b64dec(String(s)); };
    g.btoa = function (s) { return __b64enc(String(s)); };

    function URL(input, base) {
        var r = __parseURL(String(input), base === undefined ? "" : String(base));
        if (!r) throw new TypeError("Invalid URL: " + input);
        for (var k in r) this[k] = r[k];
    }
    URL.prototype.toString = function () { return this.href; };
    URL.prototype.toJSON = function () { return this.href; };
    g.URL = URL;

    // Reading a value that is unknown during preview throws, as the Node.js policy SDK's unknown-checking proxy does.
    function UnknownValueError(path) {
        this.name = "UnknownValueError";
        this.message = "string value at ." + path + " can't be known during preview";
    }
    UnknownValueError.prototype = Object.create(Error.prototype);
    UnknownValueError.prototype.constructor = UnknownValueError;
    g.__unknownGetter = function (path) {
        return function () { throw new UnknownValueError(path); };
    };

    function NotApplicable(reason) { this.reason = reason; }

    // __invoke runs one policy's validate and returns {v: [[msgOrObj, urn]...], na?: string, unk?: string}.
    g.__invoke = function (fn, input) {
        var v = [];
        var res = { v: v };
        var ctx = {
            reportViolation: function (m, urn) { v.push([m, urn]); },
            notApplicable: function (reason) { throw new NotApplicable(reason); },
        };
        try {
            fn(input, ctx);
        } catch (e) {
            if (e instanceof NotApplicable) {
                res.na = e.reason === undefined || e.reason === null ? "" : String(e.reason);
            } else if (e && e.name === "UnknownValueError") {
                res.unk = e.message;
            } else {
                throw e;
            }
        }
        return res;
    };

    g.__describe = function () {
        var p = g.__pulumiPolicyPack;
        if (!p || typeof p !== "object") {
            throw new Error("policy pack bundle did not set globalThis.__pulumiPolicyPack");
        }
        return JSON.stringify(p, function (k, val) { return k === "validate" ? undefined : val; });
    };
})(globalThis);
