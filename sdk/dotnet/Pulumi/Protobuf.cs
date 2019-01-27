using System;
using System.Linq;
using System.Collections.Generic;
using Google.Protobuf.WellKnownTypes;

namespace Pulumi {

    public interface IProtobuf {
        Value ToProtobuf();
    }
    public interface IIOProtobuf {
        IO<Value> ToProtobuf();
    }

    public static class Protobuf {
        public static Value ToProtobuf(IProtobuf value) {
            if( value == null) {
                return Value.ForNull();
            } else {
                return value.ToProtobuf();
            }
        }

        public static Value ToProtobuf(string value) {
            if (value == null) {
                return Value.ForNull();
            } else {
                return Value.ForString(value);
            }
        }

        public static Value ToProtobuf(int value) {
            return Value.ForNumber(value);
        }

        public static Value ToProtobuf(int? value) {
            if (!value.HasValue) {
                return Value.ForNull();
            } else {
                return Value.ForNumber(value.Value);
            }
        }
        public static Value ToProtobuf(double value) {
            return Value.ForNumber(value);
        }

        public static Value ToProtobuf(double? value) {
            if (!value.HasValue) {
                return Value.ForNull();
            } else {
                return Value.ForNumber(value.Value);
            }
        }
        public static Value ToProtobuf(bool value) {
            return Value.ForBool(value);
        }

        public static Value ToProtobuf(bool? value) {
            if (!value.HasValue) {
                return Value.ForNull();
            } else {
                return Value.ForBool(value.Value);
            }
        }

        public static Value ToProtobuf<T>(T[] value, Func<T, Value> selector) {
            if (value == null) {
                return Value.ForNull();
            } else {
                return Value.ForList(value.Select(selector).ToArray());
            }
        }

        public static Value ToProtobuf(Dictionary<string, string> value) {
            if (value == null) {
                return Value.ForNull();
            } else {
                var result = new Struct();
                foreach(var field in value) {
                    result.Fields[field.Key] = Value.ForString(field.Value);
                }
                return Value.ForStruct(result);
            }
        }

        public static Value ToProtobuf(params KeyValuePair<string, Value>[] fields) {
            if (fields == null) {
                return Value.ForNull();
            } else {
                var result = new Struct();
                foreach(var field in fields) {
                    result.Fields[field.Key] = field.Value;
                }
                return Value.ForStruct(result);
            }
        }

        // ToProtobuf IO
        public static IO<Value> ToProtobuf(IIOProtobuf value) {
            if( value == null) {
                return null;
            } else {
                return value.ToProtobuf();
            }
        }

        public static IO<Value> ToProtobuf<T>(IO<T> value) where T : IIOProtobuf {
            if( value == null) {
                return null;
            } else {
                return value.SelectMany(item => item.ToProtobuf());
            }
        }

        public static IO<Value> ToProtobuf(IO<string> value) {
            if (value == null) {
                return null;
            } else {
                return value.Select(item => ToProtobuf(item));
            }
        }

        public static IO<Value> ToProtobuf(IO<int> value) {
            if (value == null) {
                return null;
            } else {
                return value.Select(item => ToProtobuf(item));
            }
        }

        public static IO<Value> ToProtobuf(IO<double> value) {
            if (value == null) {
                return null;
            } else {
                return value.Select(item => ToProtobuf(item));
            }
        }

        public static IO<Value> ToProtobuf(IO<bool> value) {
            if (value == null) {
                return null;
            } else {
                return value.Select(item => ToProtobuf(item));
            }
        }

        public static IO<Value> ToProtobuf<T>(IO<T[]> value, Func<T, IO<Value>> selector) {
            if (value == null) {
                return null;
            } else {
                return value.SelectMany(item => {
                    if (item == null) {
                        return Value.ForNull();
                    } else {
                        IEnumerable<IO<Value>> ioList = item.Select(selector);
                        IO<Value[]> listIo = IO.WhenAll(ioList);
                        IO<Value> result = listIo.Select(list => Value.ForList(list));
                        return result;
                    }
                });
            }
        }

        public static IO<Value> ToProtobuf(IO<Dictionary<string, string>> value) {
            if (value == null) {
                return null;
            } else {
                return value.Select(item => ToProtobuf(item));
            }
        }

        public static IO<Value> ToProtobuf(params KeyValuePair<string, IO<Value>>[] fields) {
            if (fields == null) {
                return Value.ForNull();
            } else {
                var ioFields = fields.Select(kv => kv.Value.Select(value => new KeyValuePair<string, Value>(kv.Key, value)));
                return IO.WhenAll(ioFields).Select(ToProtobuf);
            }
        }

        ///////////////
        // FromProtobuf
        ///////////////

        public static string ToString(Value value) {
            return value.StringValue;
        }

        public static int ToInt(Value value) {
            return (int)value.NumberValue;
        }

        public static double ToDouble(Value value) {
            return value.NumberValue;
        }

        public static bool ToBool(Value value) {
            return value.BoolValue;
        }

        public static T[] ToList<T>(Value value, Func<Value, T> selector) {
            return value.ListValue.Values.Select(selector).ToArray();
        }

        public static Dictionary<string, string> ToMap(Value value) {
            var structValue = value.StructValue;
            var result = new Dictionary<string, string>();
            foreach(var field in structValue.Fields) {
                result[field.Key] = field.Value.StringValue;
            }
            return result;
        }
    }
}