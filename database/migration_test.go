package database

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

const originalSchema = `
PRAGMA foreign_keys=OFF;
BEGIN TRANSACTION;
CREATE TABLE version (version INT);
INSERT INTO version VALUES(1);
CREATE TABLE swaps (id VARCHAR PRIMARY KEY, status VARCHAR , privateKey VARCHAR, preimage VARCHAR, redeemScript VARCHAR, invoice VARCHAR, address VARCHAR, expectedAmount INT, timeoutBlockheight INTEGER, lockupTransactionId VARCHAR, refundTransactionId VARCHAR);
CREATE TABLE reverseSwaps (id VARCHAR PRIMARY KEY, status VARCHAR, acceptZeroConf BOOLEAN, privateKey VARCHAR, preimage VARCHAR, redeemScript VARCHAR, invoice VARCHAR, claimAddress VARCHAR, expectedAmount INT, timeoutBlockheight INTEGER, lockupTransactionId VARCHAR, claimTransactionId VARCHAR);
CREATE TABLE channelCreations (swapId VARCHAR PRIMARY KEY, status VARCHAR, inboundLiquidity INT, private BOOLEAN, fundingTransactionId VARCHAR, fundingTransactionVout INT);
COMMIT;
  `

const pendingSchema = `
PRAGMA foreign_keys=OFF;
BEGIN TRANSACTION;
CREATE TABLE version (version INT);
INSERT INTO version VALUES(2);
CREATE TABLE macaroons (id VARCHAR PRIMARY KEY, rootKey VARCHAR);
INSERT INTO macaroons VALUES('30','b29a3cbc4aa1b015d1a0ea2f56e790266c31ea81cbd08cc76e06d9dd5441c62d');
CREATE TABLE swaps (id VARCHAR PRIMARY KEY, state INT, error VARCHAR, status VARCHAR, privateKey VARCHAR, preimage VARCHAR, redeemScript VARCHAR, invoice VARCHAR, address VARCHAR, expectedAmount INT, timeoutBlockheight INTEGER, lockupTransactionId VARCHAR, refundTransactionId VARCHAR);
INSERT INTO swaps VALUES('wueZB5',0,'','swap.created','e411ec97f33f5801902ed6ba21367fdf3c0ed3ede606b8f2412bec321f7d0421','aa6f7a54744c5e35313800813de0c92cfa94574fcdae65f9c9cb1f807585828b','a914ab060bdd90d4c6dedd5907e1e6cae3f8e724131587632103defe74e5f8393f9c48d9c9fb0bf49a883adac25269890bb1d2d7c41af619f2d567020f01b17521021d69e84e2bec2c952f505afbb777762e9fd14bfa6c3dc9d47ed90ae6228c023a68ac','','2ND2qxAKMjaMLAjcrGFJdtPC77iHKXt5ECU',0,271,'','');
INSERT INTO swaps VALUES('ZCP4qU',0,'','swap.created','3f20a7387831acbbb2abc192ca29a7c0e3c9557e0edc91456cde6438c56520a6','a3c2bf01a36d6a22d743c15a71a66c909ab94840454fabb827aec36ba809841d','a9145d63a011783b31e679f14ea046032352a152a1da87632103611b80e6aa832718caae89c59f16576888db6f911f88c2d1fc3533bee7efc61f67020f01b1752102992bfe89a9027cf24777fabc55f4f877d6543dae5e957fc05059bea08bf6577e68ac','','2N7Zsot7mr6N1ntx1GsXnV65EQSaQj5nPzA',0,271,'','');
INSERT INTO swaps VALUES('4TsI8c',0,'','swap.created','f8bfe030191be5b54d7e405b1668a207daf54b4ec33a3aa550fa1286eb376d47','2dedaa4a14820942757a13186252cff391cddbe3ab65b895940544f8714e3eec','a914b5adc7e0484354571f362f7556c626372210c7f387632102d9fa65e27683dd2f1924e93508e5e99133e268c578733263eb7a891f6778a43a67020f01b1752103555c90ce5965f0f4c3f4ace91fc6f5746e46820147ea9880a98aec298b7e7b5268ac','','2NDKcAYyLVMRWkxuBTcvinzCM3x6iuWDYnE',0,271,'','');
CREATE TABLE reverseSwaps (id VARCHAR PRIMARY KEY, state INT, error VARCHAR, status VARCHAR, acceptZeroConf BOOLEAN, privateKey VARCHAR, preimage VARCHAR, redeemScript VARCHAR, invoice VARCHAR, claimAddress VARCHAR, expectedAmount INT, timeoutBlockheight INTEGER, lockupTransactionId VARCHAR, claimTransactionId VARCHAR);
INSERT INTO reverseSwaps VALUES('cDMsZ7',0,'','transaction.mempool',0,'6dcb3738b74e4786d612a41d8becb970e9a942501e1eb7735e1beba9ca810209','d2de217a189c20e2fac1e84392791c747015b3d7ff75446785f06121511e9400','8201208763a9148b1cb35494664a6dcd240cf4f6b7ebbfaae9506688210306e7e16aa947fbff50a538dd0641bce6c031a0188431864af19d0d5b423cde4c6775023b01b17521035578a38b772461f2481b2a9c6f6802419b11282fb3719cde6af337c077e3d5f368ac','lnbcrt1231230n1pjlpy3kpp5at39p9586hqnhlgn8tk7yls89hm7qsk0fuyfq2mgy2yh3z8zxn0sdql2djkuepqw3hjqsj5gvsxzerywfjhxuccqzylxqyp2xqsp54mcn3pmt3f4tw2c0nvqfrr8dr66jtgkdlffzpxw6g5kl77gk6q2s9qyyssqafnpa95zch83w2wsev3nvt572vx6x3epakdssmr95yg9ut7vrn8xuwvfj74fdxh0zazd89wm3l3g575n47jvmpg76z3g3k46nh8ln0qp0qqk4n','bcrt1qz45kycre4ecry5d6s2fecsjq02wklhv29yvnqe',122201,315,'','');
CREATE TABLE channelCreations (swapId VARCHAR PRIMARY KEY, status VARCHAR, inboundLiquidity INT, private BOOLEAN, fundingTransactionId VARCHAR, fundingTransactionVout INT);
COMMIT;
  `

func TestMigration(t *testing.T) {
	tt := []struct {
		name        string
		schema      string
		successfull bool
	}{
		{"Original", originalSchema, true},
		{"PendingSwaps", pendingSchema, false},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			path := t.TempDir() + "/test.db"
			db, err := sql.Open("sqlite3", path)
			require.NoError(t, err)
			_, err = db.Exec(tc.schema)
			require.NoError(t, err)
			database := &Database{Path: path, db: db}
			originalVersion, err := database.queryVersion()
			require.NoError(t, err)
			db.Close()

			database = &Database{Path: path}
			migrationError := database.Connect()
			version, err := database.queryVersion()
			require.NoError(t, err)
			if tc.successfull {
				require.NoError(t, migrationError)
				require.Equal(t, latestSchemaVersion, version)
			} else {
				require.Error(t, migrationError)
				require.Equal(t, originalVersion, version)
			}
		})

	}
}
