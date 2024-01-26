package wallet

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const data = "aösldfjaösldkfösldkfjaösldkfj"
const password = "hallo123"

func TestEncryptDecrypt(t *testing.T) {
	salt, err := generateSalt()
	require.NoError(t, err)
	require.NotEmpty(t, salt)

	cipher, err := encrypt(data, password, salt)
	require.NoError(t, err)

	plain, err := decrypt(cipher, password, salt)
	require.NoError(t, err)
	require.Equal(t, data, plain)

	_, err = decrypt(cipher, "wrong", salt)
	require.Error(t, err)

	salt, err = generateSalt()
	require.NoError(t, err)
	require.NotEmpty(t, salt)
	_, err = decrypt(cipher, password, salt)
	require.Error(t, err)
}
