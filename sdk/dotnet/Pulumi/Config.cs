//using System;
//using System.Collections.Generic;
//using Newtonsoft.Json;

//namespace Pulumi {
//    public class Config {
//        private string m_prefix;

//        private static Dictionary<string, string> s_config = loadConfig();

//        static Dictionary<string, string> loadConfig() {
//            string envValue = Environment.GetEnvironmentVariable("PULUMI_CONFIG");
//            if (envValue != null) {
//                return JsonConvert.DeserializeObject<Dictionary<string, string>>(envValue);
//            }

//            return new Dictionary<string, string>();
//        }

//        public Config(string name) {
//            m_prefix = name;
//        }

//        private string FullKey(string name) {
//            return m_prefix + ":" + name;
//        }

//        public string this[string name] {
//            get { 
//                return s_config[FullKey(name)];
//            }
//        }
//    }
//}