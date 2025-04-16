package database

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration(t *testing.T) {
	tt := []struct {
		name        string
		schema      string
		successfull bool
	}{
		{"Original", originalSchema, true},
		{"PendingSwaps", pendingSchema, false},
		{"Full", fullSchema, true},
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
			require.NoError(t, db.Close())

			database = &Database{Path: path}
			migrationError := database.Connect()
			version, err := database.queryVersion()
			require.NoError(t, err)
			if tc.successfull {
				require.NoError(t, migrationError)
				require.Equal(t, latestSchemaVersion, version)
				_, err := database.QuerySwaps(SwapQuery{})
				require.NoError(t, err)
				reverseSwaps, err := database.QueryReverseSwaps(SwapQuery{})
				require.NoError(t, err)
				for _, reverseSwap := range reverseSwaps {
					require.NotZero(t, reverseSwap.InvoiceAmount)
				}
				_, err = database.QueryWalletCredentials()
				require.NoError(t, err)
			} else {
				require.Error(t, migrationError)
				require.Equal(t, originalVersion, version)
			}
		})

	}
}

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

const fullSchema = `
PRAGMA foreign_keys=OFF;
BEGIN TRANSACTION;
CREATE TABLE version
(
    version INT
);
INSERT INTO version VALUES(4);
CREATE TABLE macaroons
(
    id      VARCHAR PRIMARY KEY,
    rootKey VARCHAR
);
INSERT INTO macaroons VALUES('30','cede425f2a064f4117564259d14cd7d6efdfb96c6ed98938f300103f861894b6');
CREATE TABLE swaps
(
    id                  VARCHAR PRIMARY KEY,
    fromCurrency        VARCHAR,
    toCurrency          VARCHAR,
    chanIds             JSON,
    state               INT,
    error               VARCHAR,
    status              VARCHAR,
    privateKey          VARCHAR,
    swapTree            JSON,
    claimPubKey         VARCHAR,
    preimage            VARCHAR,
    redeemScript        VARCHAR,
    invoice             VARCHAR,
    address             VARCHAR,
    expectedAmount      INT,
    timeoutBlockheight  INTEGER,
    lockupTransactionId VARCHAR,
    refundTransactionId VARCHAR,
    refundAddress       VARCHAR DEFAULT '',
    blindingKey         VARCHAR,
    isAuto              BOOLEAN DEFAULT 0,
    serviceFee          INT,
    serviceFeePercent   REAL,
    onchainFee          INT,
    createdAt           INT,
    autoSend            BOOLEAN
);
INSERT INTO swaps VALUES('L3RBYSeQhQbh','BTC','BTC','null',1,'','transaction.claimed','d7971f4e7b5916251a789be9ba5a33a85769942c769ca2c52e62dcf85fdba80d','{"claimLeaf":{"version":192,"output":"a914a241d077ee6d57b783247dffd56cdb8e85830aa18820defe74e5f8393f9c48d9c9fb0bf49a883adac25269890bb1d2d7c41af619f2d5ac"},"refundLeaf":{"version":192,"output":"2034015d554c831fc483f25e1fd632d9e2116697e569d5199cad67fcff6bff34f6ad021b01b1"}}','03defe74e5f8393f9c48d9c9fb0bf49a883adac25269890bb1d2d7c41af619f2d5','65d6c651b4ecc6d22b5c1ba416a6537517ad4d15ccfa5e6fedee6c530e521bde','','lnbcrt23133330n1pnq9x6usp5lg8sdnf32chqurwypntasyw2fddfznnz2l8dmg85eg2ma370wdhspp5j72pt575c70gh4yfg9hfn64anlysnh697mwwn8nxalf90gec9agsdp92d6kymtpwf5kuefq2dmkzupqveex7mfqgf2yxxq8pnq8pducqp5rzjqfrrywavujdhg4ue38948c35zkwwsashhahlw6jygn2c6r4echz3cqqq5cqqqqgqqqqqqqlgqqqqqqgq2q9qxpqysgqfaznpau5tz9rezz45xm9gkvslmhckeufn93l786qxenfeyps4gysmr5eahvnsam29j0mzeu4qpjrey5aqwt84asy69h48yq3csktgpcqtn9fv9','bcrt1pryhu8j58yt338hrxvv2h9zg6hm8zqty0vqg74hfvp48jfyacw89q8ra2vc',2340000,283,'da143167c3f463377a5b8fae83feb00ecf313eb4a7ec6286360fc6934ac441a3','','','',0,11700,0.5,14967,1711446848,0);
INSERT INTO swaps VALUES('7VonTzH8PpX9','L-BTC','BTC','null',1,'','transaction.claimed','f143ed9d0dd57d5a0501e519f418f098aaba6e172fe304e367ac9bac8bf280f6','{"claimLeaf":{"version":196,"output":"a9147dd1bd203ac57b6354babf3d30d69f4e977d23238820c4b06805b2103b001673228719c7605d12072d2eaee379b7403f4cd81c2202fbac"},"refundLeaf":{"version":196,"output":"20646797de4e268a9fbfc349ef78a0c767dbe5ccada7d8f6fdde51243cbad4c016ad02c201b1"}}','02c4b06805b2103b001673228719c7605d12072d2eaee379b7403f4cd81c2202fb','','','lnbcrt500u1pnq9xa7sp5duec7ufz07grpt49huxwnydadzmz8jkvfrk6uh72leh60pdy3tkspp5j5ste694x2khuwru5npgm49w300v7twer6p7624dj20tv8d2k7msdpg2d6kymtpwf5kuefq2dmkzupqveex7mfqfsk5y4zrxqyjw5qcqp5rzjqdqdlefhqzcrh2wq266nt3jt5sqg9axlrz8fhcdq9a384r885mp3cqqq5qqqqqgqqqqqqqlgqqqqqqgq2q9qxpqysgq7sg5wwyavqkvlpvdd7n8uqwlrxw50qkvcp8gfk2upapu5cfmq8zq3c8lr3tln0x8t82e5p0h9rpf2u6ehnyq22ractl8gnyk68f7xxgpclter4','el1pqv0xqk9kzmtxz9dpq764d96apjf2lw0vp3uvr9qnv0qyn78tfwjxh9n8mxjw3al7luk6v3mz2ra0r4pudz5xflfjpl03prpajy0zwsdjf0kv424tyrgu',50198,450,'2208debc3aee35a9dd80584d9349391122e9434fc18fba14de053f49b61d5200','','','e7a6f6616d80bbb0e2f6df9644a1ff050d56afa065053e811592127b4c54e8cd',0,50,0.10000000149011611938,430,1711446974,1);
CREATE TABLE reverseSwaps
(
    id                  VARCHAR PRIMARY KEY,
    fromCurrency        VARCHAR,
    toCurrency          VARCHAR,
    chanIds             JSON,
    state               INT,
    error               VARCHAR,
    status              VARCHAR,
    acceptZeroConf      BOOLEAN,
    privateKey          VARCHAR,
    swapTree            JSON,
    refundPubKey        VARCHAR,
    preimage            VARCHAR,
    redeemScript        VARCHAR,
    invoice             VARCHAR,
    claimAddress        VARCHAR,
    expectedAmount      INT,
    timeoutBlockheight  INTEGER,
    lockupTransactionId VARCHAR,
    claimTransactionId  VARCHAR,
    blindingKey         VARCHAR,
    isAuto              BOOLEAN DEFAULT 0,
    routingFeeMsat      INT,
    serviceFee          INT,
    serviceFeePercent   REAL    DEFAULT 0,
    onchainFee          INT,
    createdAt           INT
);
INSERT INTO reverseSwaps VALUES('UTqr69UXZwPb','BTC','BTC','null',1,'','invoice.settled',1,'f5bfff443738245e3916409916ba66b096b26604c7e3614400275b9e2e3dced5','{"claimLeaf":{"version":192,"output":"82012088a9140a70aadd80be3ec71ce5996ae68db578b12a12ba8820aceb08b7879817cb2d16fb58b0f9b278e8bca8f31b0a4f0c09b449bfad5e11d1ac"},"refundLeaf":{"version":192,"output":"20611b80e6aa832718caae89c59f16576888db6f911f88c2d1fc3533bee7efc61fad024901b1"}}','03611b80e6aa832718caae89c59f16576888db6f911f88c2d1fc3533bee7efc61f','cb8b6fbe62dd52ef17bf7f5697ec64ddc401836c4f4c21c9bf93c4fcaeeb2935','','lnbcrt1m1pnq9xmnpp59snapt6ks5l8ktv2xhh6snq8pcnxd0cyufa9vq5n208ug88vatksdql2djkuepqw3hjqsj5gvsxzerywfjhxuccqzylxqyp2xqsp5vwgc9zhr5taqlwfplexxs9m4g3c4edct3dph629727jquk6ssr6q9qyyssqhcwjkj60c0xnfxqph9nscw5hsxm6wudwae0ndnf2835q3peaxv0qwyazfnx76jpm7tcyh2edsvda2ycx0vy6pv2pq62fkq4z30u9wscppw5a54','bcrt1ql5tqxm2nun8qkkdtfzq8uvhrl4tqlhu4vkdnpv',84100,329,'34c10f75b375cb84d84e032ec9dad0c1c1375dfb3076e97935e3cde2f497f17e','7c8a3dbcd3baad81c1f3bc916d741f3f2397f62bdab572fdb7e807742fc5721a','',0,0,500,0.5,15508,1711446899);
INSERT INTO reverseSwaps VALUES('yPzrSdIyXpXS','BTC','L-BTC','null',1,'','invoice.settled',1,'9dfae96ab07360f5c5812d7fa7c3beeb3fea5f33ae34183c3bd9c518d222663a','{"claimLeaf":{"version":196,"output":"82012088a9142292c0e6754483e4bb81dfb3fc7b68a08aa84ac888200698009a99d1d2c927992456e303b3440fc152f86f0df6df7d493a4af350a2a7ac"},"refundLeaf":{"version":196,"output":"20f80e5650435fb598bb07257d50af378d4f7ddf8f2f78181f8b29abb0b05ecb47ad023606b1"}}','03f80e5650435fb598bb07257d50af378d4f7ddf8f2f78181f8b29abb0b05ecb47','2a9eba1c500cc25ef530f0bb3c0ca11b653cbb4fbe6e01ebe2053d8842bbe2b1','','lnbcrt1231230n1pnq9xu7pp5wjvu49xjyhuc44jstlct64t0n447fgrtw80zlzsqffayluc0dl3sdpz2djkuepqw3hjqnpdgf2yxgrpv3j8yetnwvcqz95xqyp2xqsp504e3n0pznk48zkp0nwaksxm2nmzpy78wcpr2cvdgh6x4g7zkclzq9qyyssq9ks69lywut0qvvkfhqhd697s8l4gvcsjhwwktx6599z28acns248hj2t0v32088rs50ms7wn74geunejkse443rfjwsgsuaym9xvgesq5jxpx8','el1qq27t5a5qj4ry07hqrqqff6qmxzfehauvrrenz7zz52kfkpsvydukxktxkvc48m439jjvehc8c720eypysz5eqevvm47728hfu',122723,1590,'e49af27913e56f8c92fb6b5f631dc8504f6c9627ac07d234c4c247cb5bbe0977','a733863921751f3075d55b577f4d033ccf534a0553635a878aa491a3e380a5af','b4b986874154322cfd5d48f88f2b9267fba317975b14a8c729b12b684ef3ce7b',0,0,123,0.10000000149011611938,419,1711446942);
CREATE TABLE autobudget
(
    startDate INTEGER PRIMARY KEY,
    endDate   INTEGER
);
CREATE TABLE wallets
(
    name           VARCHAR PRIMARY KEY,
    currency       VARCHAR,
    xpub           VARCHAR,
    coreDescriptor VARCHAR,
    mnemonic       VARCHAR,
    subaccount     INT,
    salt           VARCHAR
);
INSERT INTO wallets VALUES('liquid','L-BTC','','','61d4de377fbaa66788bc1deafef378e89edb8cdf638cf3b480077e7f35a5b2ee1ebffb8feaa877533a1c6b80f7de49c695574b7d0b25d1b7f98c055cc8215469b1d0a28b3cf9e779e86295e01de7bb025bc7c4be9d535ae4d88aca7cee',1,'2dc6cdf52309336820ae784e42f5b31f8e237d396bd6ae6ee1431cedaa773267');
COMMIT;
`
