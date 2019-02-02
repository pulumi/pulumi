using System;
using System.Collections.Generic;
using Newtonsoft.Json;

namespace Pulumi {
    public class Config {
        
        /// name is the configuration bag's logical name and uniquely identifies it.  The default is the name of the current
        /// project.
        public readonly string Name;

        private static Dictionary<string, string> s_config = loadConfig();

        static Dictionary<string, string> loadConfig() {
            string envValue = Environment.GetEnvironmentVariable("PULUMI_CONFIG");
            if (envValue != null) {
                return JsonConvert.DeserializeObject<Dictionary<string, string>>(envValue);
            }

            return new Dictionary<string, string>();
        }

        public Config(string name = null) {
            if (name == null) {
                name = Runtime.Project;
            }

            if (name.EndsWith(":config")) {
                name = name.Replace(":config", "");
            }

            Name = name;
        }

        private string FullKey(string key) {
            return Name + ":" + key;
        }

        public string Get(string key) {
            return s_config[FullKey(key)];
        }

        /// require loads a configuration value by its given key.  If it doesn't exist, an error is thrown.
        public string Require(string key) {
            var v = Get(key);
            if (v == null) {
                throw new Exception(FullKey(key));
            }
            return v;
        }
    }
}