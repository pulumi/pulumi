
---
title: "Foo"
title_tag: "repro.Foo"
meta_desc: "Documentation for the repro.Foo resource with examples, input properties, output properties, lookup functions, and supporting types."
layout: api
no_edit_this_page: true
---



<!-- WARNING: this file was generated by test. -->
<!-- Do not edit by hand unless you're certain you know what you are doing! -->




## Create Foo Resource {#create}

Resources are created with functions called constructors. To learn more about declaring and configuring resources, see [Resources](/docs/concepts/resources/).

### Constructor syntax
<div>
<pulumi-chooser type="language" options="typescript,python,go,csharp,java,yaml"></pulumi-chooser>
</div>


<div>
<pulumi-choosable type="language" values="javascript,typescript">
<div class="no-copy"><div class="highlight"><pre class="chroma"><code class="language-typescript" data-lang="typescript"><span class="k">new </span><span class="nx">Foo</span><span class="p">(</span><span class="nx">name</span><span class="p">:</span> <span class="nx">string</span><span class="p">,</span> <span class="nx">args</span><span class="p">?:</span> <span class="nx"><a href="#inputs">FooArgs</a></span><span class="p">,</span> <span class="nx">opts</span><span class="p">?:</span> <span class="nx"><a href="/docs/reference/pkg/nodejs/pulumi/pulumi/#CustomResourceOptions">CustomResourceOptions</a></span><span class="p">);</span></code></pre></div>
</div></pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="python">
<div class="no-copy"><div class="highlight"><pre class="chroma"><code class="language-python" data-lang="python"><span class=nd>@overload</span>
<span class="k">def </span><span class="nx">Foo</span><span class="p">(</span><span class="nx">resource_name</span><span class="p">:</span> <span class="nx">str</span><span class="p">,</span>
        <span class="nx">args</span><span class="p">:</span> <span class="nx"><a href="#inputs">Optional[FooArgs]</a></span> = None<span class="p">,</span>
        <span class="nx">opts</span><span class="p">:</span> <span class="nx"><a href="/docs/reference/pkg/python/pulumi/#pulumi.ResourceOptions">Optional[ResourceOptions]</a></span> = None<span class="p">)</span>
<span></span>
<span class=nd>@overload</span>
<span class="k">def </span><span class="nx">Foo</span><span class="p">(</span><span class="nx">resource_name</span><span class="p">:</span> <span class="nx">str</span><span class="p">,</span>
        <span class="nx">opts</span><span class="p">:</span> <span class="nx"><a href="/docs/reference/pkg/python/pulumi/#pulumi.ResourceOptions">Optional[ResourceOptions]</a></span> = None<span class="p">)</span></code></pre></div>
</div></pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="go">
<div class="no-copy"><div class="highlight"><pre class="chroma"><code class="language-go" data-lang="go"><span class="k">func </span><span class="nx">NewFoo</span><span class="p">(</span><span class="nx">ctx</span><span class="p"> *</span><span class="nx"><a href="https://pkg.go.dev/github.com/pulumi/pulumi/sdk/v3/go/pulumi?tab=doc#Context">Context</a></span><span class="p">,</span> <span class="nx">name</span><span class="p"> </span><span class="nx">string</span><span class="p">,</span> <span class="nx">args</span><span class="p"> *</span><span class="nx"><a href="#inputs">FooArgs</a></span><span class="p">,</span> <span class="nx">opts</span><span class="p"> ...</span><span class="nx"><a href="https://pkg.go.dev/github.com/pulumi/pulumi/sdk/v3/go/pulumi?tab=doc#ResourceOption">ResourceOption</a></span><span class="p">) (*<span class="nx">Foo</span>, error)</span></code></pre></div>
</div></pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="csharp">
<div class="no-copy"><div class="highlight"><pre class="chroma"><code class="language-csharp" data-lang="csharp"><span class="k">public </span><span class="nx">Foo</span><span class="p">(</span><span class="nx">string</span><span class="p"> </span><span class="nx">name<span class="p">,</span> <span class="nx"><a href="#inputs">FooArgs</a></span><span class="p">? </span><span class="nx">args = null<span class="p">,</span> <span class="nx"><a href="/docs/reference/pkg/dotnet/Pulumi/Pulumi.CustomResourceOptions.html">CustomResourceOptions</a></span><span class="p">? </span><span class="nx">opts = null<span class="p">)</span></code></pre></div>
</div></pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="java">
<div class="no-copy"><div class="highlight"><pre class="chroma">
<code class="language-java" data-lang="java"><span class="k">public </span><span class="nx">Foo</span><span class="p">(</span><span class="nx">String</span><span class="p"> </span><span class="nx">name<span class="p">,</span> <span class="nx"><a href="#inputs">FooArgs</a></span><span class="p"> </span><span class="nx">args<span class="p">)</span>
<span class="k">public </span><span class="nx">Foo</span><span class="p">(</span><span class="nx">String</span><span class="p"> </span><span class="nx">name<span class="p">,</span> <span class="nx"><a href="#inputs">FooArgs</a></span><span class="p"> </span><span class="nx">args<span class="p">,</span> <span class="nx">CustomResourceOptions</span><span class="p"> </span><span class="nx">options<span class="p">)</span>
</code></pre></div></div>
</pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="yaml">
<div class="no-copy"><div class="highlight"><pre class="chroma"><code class="language-yaml" data-lang="yaml">type: <span class="nx">repro:Foo</span><span class="p"></span>
<span class="p">properties</span><span class="p">: </span><span class="c">#&nbsp;The arguments to resource properties.</span>
<span class="p"></span><span class="p">options</span><span class="p">: </span><span class="c">#&nbsp;Bag of options to control resource&#39;s behavior.</span>
<span class="p"></span>
</code></pre></div></div>
</pulumi-choosable>
</div>

#### Parameters

<div>
<pulumi-choosable type="language" values="javascript,typescript">

<dl class="resources-properties"><dt
        class="property-required" title="Required">
        <span>name</span>
        <span class="property-indicator"></span>
        <span class="property-type">string</span>
    </dt>
    <dd>The unique name of the resource.</dd><dt
        class="property-optional" title="Optional">
        <span>args</span>
        <span class="property-indicator"></span>
        <span class="property-type"><a href="#inputs">FooArgs</a></span>
    </dt>
    <dd>The arguments to resource properties.</dd><dt
        class="property-optional" title="Optional">
        <span>opts</span>
        <span class="property-indicator"></span>
        <span class="property-type"><a href="/docs/reference/pkg/nodejs/pulumi/pulumi/#CustomResourceOptions">CustomResourceOptions</a></span>
    </dt>
    <dd>Bag of options to control resource&#39;s behavior.</dd></dl>

</pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="python">

<dl class="resources-properties"><dt
        class="property-required" title="Required">
        <span>resource_name</span>
        <span class="property-indicator"></span>
        <span class="property-type">str</span>
    </dt>
    <dd>The unique name of the resource.</dd><dt
        class="property-optional" title="Optional">
        <span>args</span>
        <span class="property-indicator"></span>
        <span class="property-type"><a href="#inputs">FooArgs</a></span>
    </dt>
    <dd>The arguments to resource properties.</dd><dt
        class="property-optional" title="Optional">
        <span>opts</span>
        <span class="property-indicator"></span>
        <span class="property-type"><a href="/docs/reference/pkg/python/pulumi/#pulumi.ResourceOptions">ResourceOptions</a></span>
    </dt>
    <dd>Bag of options to control resource&#39;s behavior.</dd></dl>

</pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="go">

<dl class="resources-properties"><dt
        class="property-optional" title="Optional">
        <span>ctx</span>
        <span class="property-indicator"></span>
        <span class="property-type"><a href="https://pkg.go.dev/github.com/pulumi/pulumi/sdk/v3/go/pulumi?tab=doc#Context">Context</a></span>
    </dt>
    <dd>Context object for the current deployment.</dd><dt
        class="property-required" title="Required">
        <span>name</span>
        <span class="property-indicator"></span>
        <span class="property-type">string</span>
    </dt>
    <dd>The unique name of the resource.</dd><dt
        class="property-optional" title="Optional">
        <span>args</span>
        <span class="property-indicator"></span>
        <span class="property-type"><a href="#inputs">FooArgs</a></span>
    </dt>
    <dd>The arguments to resource properties.</dd><dt
        class="property-optional" title="Optional">
        <span>opts</span>
        <span class="property-indicator"></span>
        <span class="property-type"><a href="https://pkg.go.dev/github.com/pulumi/pulumi/sdk/v3/go/pulumi?tab=doc#ResourceOption">ResourceOption</a></span>
    </dt>
    <dd>Bag of options to control resource&#39;s behavior.</dd></dl>

</pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="csharp">

<dl class="resources-properties"><dt
        class="property-required" title="Required">
        <span>name</span>
        <span class="property-indicator"></span>
        <span class="property-type">string</span>
    </dt>
    <dd>The unique name of the resource.</dd><dt
        class="property-optional" title="Optional">
        <span>args</span>
        <span class="property-indicator"></span>
        <span class="property-type"><a href="#inputs">FooArgs</a></span>
    </dt>
    <dd>The arguments to resource properties.</dd><dt
        class="property-optional" title="Optional">
        <span>opts</span>
        <span class="property-indicator"></span>
        <span class="property-type"><a href="/docs/reference/pkg/dotnet/Pulumi/Pulumi.CustomResourceOptions.html">CustomResourceOptions</a></span>
    </dt>
    <dd>Bag of options to control resource&#39;s behavior.</dd></dl>

</pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="java">

<dl class="resources-properties"><dt
        class="property-required" title="Required">
        <span>name</span>
        <span class="property-indicator"></span>
        <span class="property-type">String</span>
    </dt>
    <dd>The unique name of the resource.</dd><dt
        class="property-required" title="Required">
        <span>args</span>
        <span class="property-indicator"></span>
        <span class="property-type"><a href="#inputs">FooArgs</a></span>
    </dt>
    <dd>The arguments to resource properties.</dd><dt
        class="property-optional" title="Optional">
        <span>options</span>
        <span class="property-indicator"></span>
        <span class="property-type">CustomResourceOptions</span>
    </dt>
    <dd>Bag of options to control resource&#39;s behavior.</dd></dl>

</pulumi-choosable>
</div>



### Example

The following reference example uses placeholder values for all [input properties](#inputs).
<div>
<pulumi-chooser type="language" options="typescript,python,go,csharp,java,yaml"></pulumi-chooser>
</div>


<div>
<pulumi-choosable type="language" values="csharp">

```csharp
var fooResource = new Repro.Foo("fooResource");
```

</pulumi-choosable>
</div>


<div>
<pulumi-choosable type="language" values="go">

```go
example, err := repro.NewFoo(ctx, "fooResource", nil)
```

</pulumi-choosable>
</div>


<div>
<pulumi-choosable type="language" values="java">

```java
var fooResource = new Foo("fooResource");
```

</pulumi-choosable>
</div>


<div>
<pulumi-choosable type="language" values="python">

```python
foo_resource = repro.Foo("fooResource")
```

</pulumi-choosable>
</div>


<div>
<pulumi-choosable type="language" values="typescript">

```typescript
const fooResource = new repro.Foo("fooResource", {});
```

</pulumi-choosable>
</div>


<div>
<pulumi-choosable type="language" values="yaml">

```yaml
type: repro:Foo
properties: {}
```

</pulumi-choosable>
</div>



## Foo Resource Properties {#properties}

To learn more about resource properties and how to use them, see [Inputs and Outputs](/docs/intro/concepts/inputs-outputs) in the Architecture and Concepts docs.

### Inputs

The Foo resource accepts the following [input](/docs/intro/concepts/inputs-outputs) properties:



<div>
<pulumi-choosable type="language" values="csharp">
<dl class="resources-properties"></dl>
</pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="go">
<dl class="resources-properties"></dl>
</pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="java">
<dl class="resources-properties"></dl>
</pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="javascript,typescript">
<dl class="resources-properties"></dl>
</pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="python">
<dl class="resources-properties"></dl>
</pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="yaml">
<dl class="resources-properties"></dl>
</pulumi-choosable>
</div>


### Outputs

All [input](#inputs) properties are implicitly available as output properties. Additionally, the Foo resource produces the following output properties:



<div>
<pulumi-choosable type="language" values="csharp">
<dl class="resources-properties"><dt class="property-"
            title="">
        <span id="id_csharp">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#id_csharp" style="color: inherit; text-decoration: inherit;">Id</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">string</span>
    </dt>
    <dd>The provider-assigned unique ID for this managed resource.</dd><dt class="property-"
            title="">
        <span id="conditionsets_csharp">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#conditionsets_csharp" style="color: inherit; text-decoration: inherit;">Condition<wbr>Sets</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type"><a href="#bar">List&lt;Immutable<wbr>Array&lt;Immutable<wbr>Array&lt;Bar&gt;&gt;&gt;</a></span>
    </dt>
    <dd></dd></dl>
</pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="go">
<dl class="resources-properties"><dt class="property-"
            title="">
        <span id="id_go">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#id_go" style="color: inherit; text-decoration: inherit;">Id</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">string</span>
    </dt>
    <dd>The provider-assigned unique ID for this managed resource.</dd><dt class="property-"
            title="">
        <span id="conditionsets_go">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#conditionsets_go" style="color: inherit; text-decoration: inherit;">Condition<wbr>Sets</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type"><a href="#bar">[][][]Bar</a></span>
    </dt>
    <dd></dd></dl>
</pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="java">
<dl class="resources-properties"><dt class="property-"
            title="">
        <span id="id_java">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#id_java" style="color: inherit; text-decoration: inherit;">id</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">String</span>
    </dt>
    <dd>The provider-assigned unique ID for this managed resource.</dd><dt class="property-"
            title="">
        <span id="conditionsets_java">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#conditionsets_java" style="color: inherit; text-decoration: inherit;">condition<wbr>Sets</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type"><a href="#bar">List&lt;List&lt;List&lt;Bar&gt;&gt;&gt;</a></span>
    </dt>
    <dd></dd></dl>
</pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="javascript,typescript">
<dl class="resources-properties"><dt class="property-"
            title="">
        <span id="id_nodejs">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#id_nodejs" style="color: inherit; text-decoration: inherit;">id</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">string</span>
    </dt>
    <dd>The provider-assigned unique ID for this managed resource.</dd><dt class="property-"
            title="">
        <span id="conditionsets_nodejs">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#conditionsets_nodejs" style="color: inherit; text-decoration: inherit;">condition<wbr>Sets</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type"><a href="#bar">Bar[][][]</a></span>
    </dt>
    <dd></dd></dl>
</pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="python">
<dl class="resources-properties"><dt class="property-"
            title="">
        <span id="id_python">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#id_python" style="color: inherit; text-decoration: inherit;">id</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">str</span>
    </dt>
    <dd>The provider-assigned unique ID for this managed resource.</dd><dt class="property-"
            title="">
        <span id="condition_sets_python">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#condition_sets_python" style="color: inherit; text-decoration: inherit;">condition_<wbr>sets</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type"><a href="#bar">Sequence[Sequence[Sequence[Bar]]]</a></span>
    </dt>
    <dd></dd></dl>
</pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="yaml">
<dl class="resources-properties"><dt class="property-"
            title="">
        <span id="id_yaml">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#id_yaml" style="color: inherit; text-decoration: inherit;">id</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">String</span>
    </dt>
    <dd>The provider-assigned unique ID for this managed resource.</dd><dt class="property-"
            title="">
        <span id="conditionsets_yaml">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#conditionsets_yaml" style="color: inherit; text-decoration: inherit;">condition<wbr>Sets</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type"><a href="#bar">List&lt;List&lt;List&lt;Property Map&gt;&gt;&gt;</a></span>
    </dt>
    <dd></dd></dl>
</pulumi-choosable>
</div>







## Supporting Types



<h4 id="bar">
Bar<pulumi-choosable type="language" values="python,go" class="inline">, Bar<wbr>Args</pulumi-choosable>
</h4>

<div>
<pulumi-choosable type="language" values="csharp">
<dl class="resources-properties"><dt class="property-optional"
            title="Optional">
        <span id="has-a-hyphen_csharp">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#has-a-hyphen_csharp" style="color: inherit; text-decoration: inherit;">Has-A-Hyphen</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">string</span>
    </dt>
    <dd></dd><dt class="property-optional"
            title="Optional">
        <span id="has.a.dot_csharp">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#has.a.dot_csharp" style="color: inherit; text-decoration: inherit;">Has.<wbr>A.<wbr>Dot</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">string</span>
    </dt>
    <dd></dd><dt class="property-optional"
            title="Optional">
        <span id="has_an_underscore_csharp">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#has_an_underscore_csharp" style="color: inherit; text-decoration: inherit;">Has_<wbr>an_<wbr>underscore</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">string</span>
    </dt>
    <dd></dd></dl>
</pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="go">
<dl class="resources-properties"><dt class="property-optional"
            title="Optional">
        <span id="has-a-hyphen_go">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#has-a-hyphen_go" style="color: inherit; text-decoration: inherit;">Has-A-Hyphen</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">string</span>
    </dt>
    <dd></dd><dt class="property-optional"
            title="Optional">
        <span id="has.a.dot_go">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#has.a.dot_go" style="color: inherit; text-decoration: inherit;">Has.<wbr>A.<wbr>Dot</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">string</span>
    </dt>
    <dd></dd><dt class="property-optional"
            title="Optional">
        <span id="has_an_underscore_go">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#has_an_underscore_go" style="color: inherit; text-decoration: inherit;">Has_<wbr>an_<wbr>underscore</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">string</span>
    </dt>
    <dd></dd></dl>
</pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="java">
<dl class="resources-properties"><dt class="property-optional"
            title="Optional">
        <span id="has_a_dot_java">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#has_a_dot_java" style="color: inherit; text-decoration: inherit;">has_<wbr>a_<wbr>dot</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">String</span>
    </dt>
    <dd></dd><dt class="property-optional"
            title="Optional">
        <span id="has_an_underscore_java">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#has_an_underscore_java" style="color: inherit; text-decoration: inherit;">has_<wbr>an_<wbr>underscore</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">String</span>
    </dt>
    <dd></dd><dt class="property-optional"
            title="Optional">
        <span id="hasahyphen_java">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#hasahyphen_java" style="color: inherit; text-decoration: inherit;">hasahyphen</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">String</span>
    </dt>
    <dd></dd></dl>
</pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="javascript,typescript">
<dl class="resources-properties"><dt class="property-optional"
            title="Optional">
        <span id="has-a-hyphen_nodejs">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#has-a-hyphen_nodejs" style="color: inherit; text-decoration: inherit;">has-a-hyphen</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">string</span>
    </dt>
    <dd></dd><dt class="property-optional"
            title="Optional">
        <span id="has.a.dot_nodejs">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#has.a.dot_nodejs" style="color: inherit; text-decoration: inherit;">has.a.dot</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">string</span>
    </dt>
    <dd></dd><dt class="property-optional"
            title="Optional">
        <span id="has_an_underscore_nodejs">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#has_an_underscore_nodejs" style="color: inherit; text-decoration: inherit;">has_<wbr>an_<wbr>underscore</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">string</span>
    </dt>
    <dd></dd></dl>
</pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="python">
<dl class="resources-properties"><dt class="property-optional"
            title="Optional">
        <span id="has_a_dot_python">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#has_a_dot_python" style="color: inherit; text-decoration: inherit;">has_<wbr>a_<wbr>dot</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">str</span>
    </dt>
    <dd></dd><dt class="property-optional"
            title="Optional">
        <span id="has_a_hyphen_python">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#has_a_hyphen_python" style="color: inherit; text-decoration: inherit;">has_<wbr>a_<wbr>hyphen</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">str</span>
    </dt>
    <dd></dd><dt class="property-optional"
            title="Optional">
        <span id="has_an_underscore_python">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#has_an_underscore_python" style="color: inherit; text-decoration: inherit;">has_<wbr>an_<wbr>underscore</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">str</span>
    </dt>
    <dd></dd></dl>
</pulumi-choosable>
</div>

<div>
<pulumi-choosable type="language" values="yaml">
<dl class="resources-properties"><dt class="property-optional"
            title="Optional">
        <span id="has-a-hyphen_yaml">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#has-a-hyphen_yaml" style="color: inherit; text-decoration: inherit;">has-a-hyphen</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">String</span>
    </dt>
    <dd></dd><dt class="property-optional"
            title="Optional">
        <span id="has.a.dot_yaml">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#has.a.dot_yaml" style="color: inherit; text-decoration: inherit;">has.a.dot</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">String</span>
    </dt>
    <dd></dd><dt class="property-optional"
            title="Optional">
        <span id="has_an_underscore_yaml">
<a data-swiftype-name="resource-property" data-swiftype-type="text" href="#has_an_underscore_yaml" style="color: inherit; text-decoration: inherit;">has_<wbr>an_<wbr>underscore</a>
</span>
        <span class="property-indicator"></span>
        <span class="property-type">String</span>
    </dt>
    <dd></dd></dl>
</pulumi-choosable>
</div>


<h2 id="package-details">Package Details</h2>
<dl class="package-details">
	<dt>Repository</dt>
	<dd><a href="">repro </a></dd>
	<dt>License</dt>
	<dd></dd>
</dl>
