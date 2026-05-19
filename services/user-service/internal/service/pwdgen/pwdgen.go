package pwdgen

import "golang.org/x/crypto/bcrypt"

func Generate(password []byte) []byte {
	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return hash
}

func Check(password []byte, hash []byte) bool {
	return bcrypt.CompareHashAndPassword(hash, password) == nil
}
