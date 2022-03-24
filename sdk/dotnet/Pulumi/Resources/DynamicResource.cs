// Copyright 2016-2019, Pulumi Corporation

using System;

namespace Pulumi
{
    public class DynamicResourceArgs : ResourceArgs
    {
        [Input("__provider", required: true)]
        public Input<string> Provider { get; set; } = null!;
    }

    public class DynamicResource : CustomResource
    {
      private static string GetTypeName(Resource resource)
      {
          var type = resource.GetType();
          var typeName = string.IsNullOrEmpty(type.Namespace) ? $"dynamic:{type.Name}" : $"dynamic/{type.Namespace}:{type.Name}";;
          return $"pulumi-dotnet:{typeName}";
      }

      private static Ibasa.Pikala.AssemblyPickleMode ByValueFilter(System.Reflection.Assembly assembly)
      {
          // Assemblies known to be used for defining dynamic providers
          var knownAssemblies = new string [] {
              "Pulumi", "System.Collections.Immutable"
          };
          var assemblyName = assembly.GetName().Name;
          if (!Array.Exists(knownAssemblies, name => name == assemblyName)) {
              return Ibasa.Pikala.AssemblyPickleMode.PickleByValue;
          }
          return Ibasa.Pikala.AssemblyPickleMode.Default;
      }

      private static ResourceArgs SetProvider(DynamicResourceProvider provider, DynamicResourceArgs? args)
      {
          if (args == null)
          {
              args = new DynamicResourceArgs();
          }

          var pickler = new Ibasa.Pikala.Pickler(ByValueFilter);
          var memoryStream = new System.IO.MemoryStream();
          pickler.Serialize(memoryStream, provider);
          var base64String = System.Convert.ToBase64String(memoryStream.ToArray());
          args.Provider = base64String;
          return args;
      }


#pragma warning disable RS0022 // Constructor make noninheritable base class inheritable
        public DynamicResource(DynamicResourceProvider provider, string name, DynamicResourceArgs? args, CustomResourceOptions? options = null)
            : base((Func<Resource, string>)GetTypeName, name, SetProvider(provider, args), options)
#pragma warning restore RS0022 // Constructor make noninheritable base class inheritable
        {
        }
    }
}
