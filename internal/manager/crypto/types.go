package crypto

// EncryptedBlob is the type alias used by ENT schemas for columns that hold
// wrapped secrets. It is functionally a []byte but the type makes the intent
// explicit at the schema layer.
type EncryptedBlob []byte
