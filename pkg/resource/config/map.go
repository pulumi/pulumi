// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package config

// Map is a bag of config stored in the settings file.
type Map map[Key]Value

// Decrypt returns the configuration as a map from module member to decrypted value.
func (m Map) Decrypt(decrypter Decrypter) (map[Key]string, error) {
	r := map[Key]string{}
	for k, c := range m {
		v, err := c.Value(decrypter)
		if err != nil {
			return nil, err
		}
		r[k] = v
	}
	return r, nil
}

// HasSecureValue returns true if the config map contains a secure (encrypted) value.
func (m Map) HasSecureValue() bool {
	for _, v := range m {
		if v.Secure() {
			return true
		}
	}

	return false
}
