package guest

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
)

func encryptedAddressFixture(t *testing.T) (publicArmor, messageArmor string) {
	t.Helper()
	entity, err := openpgp.NewEntity("Seller", "", "seller@example.test", nil)
	require.NoError(t, err)

	var public bytes.Buffer
	publicWriter, err := armor.Encode(&public, openpgp.PublicKeyType, nil)
	require.NoError(t, err)
	require.NoError(t, entity.Serialize(publicWriter))
	require.NoError(t, publicWriter.Close())

	var message bytes.Buffer
	messageWriter, err := armor.Encode(&message, "PGP MESSAGE", nil)
	require.NoError(t, err)
	plaintextWriter, err := openpgp.Encrypt(messageWriter, []*openpgp.Entity{entity}, nil, nil, nil)
	require.NoError(t, err)
	_, err = io.WriteString(plaintextWriter, `{"country":"US"}`)
	require.NoError(t, err)
	require.NoError(t, plaintextWriter.Close())
	require.NoError(t, messageWriter.Close())
	return public.String(), message.String()
}

func TestValidateAddressCiphertext_RequiresEncryptedPayloadPacket(t *testing.T) {
	publicArmor, validMessage := encryptedAddressFixture(t)
	require.NoError(t, validateAddressCiphertext(validMessage, publicArmor))

	decoded, err := armor.Decode(strings.NewReader(validMessage))
	require.NoError(t, err)
	op, err := packet.NewOpaqueReader(decoded.Body).Next()
	require.NoError(t, err)
	require.Equal(t, uint8(1), op.Tag, "first packet must be the public-key encrypted session key")

	var truncated bytes.Buffer
	truncatedWriter, err := armor.Encode(&truncated, "PGP MESSAGE", nil)
	require.NoError(t, err)
	require.NoError(t, op.Serialize(truncatedWriter))
	require.NoError(t, truncatedWriter.Close())

	err = validateAddressCiphertext(truncated.String(), publicArmor)
	require.ErrorContains(t, err, "no encrypted payload")
}
