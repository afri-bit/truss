// Package secrets keeps named secret values in a private file under the TRUSS
// home and removes their values from text. It knows nothing about pipelines.
//
// Values are stored unencrypted; see docs/decisions/0006. The package never
// returns a value inside an error message.
package secrets
