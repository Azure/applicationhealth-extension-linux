package redact

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValue_RedactsSensitiveKeyValuePairs(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"sas token env", "AZ_STORAGE_SAS_TOKEN_BASE64=abc123", "AZ_STORAGE_SAS_TOKEN_BASE64=" + Placeholder},
		{"password", "DB_PASSWORD=hunter2", "DB_PASSWORD=" + Placeholder},
		{"secret", "MY_SECRET=shh", "MY_SECRET=" + Placeholder},
		{"account key with underscore", "STORAGE_ACCOUNT_KEY=xyz", "STORAGE_ACCOUNT_KEY=" + Placeholder},
		{"api-key with dash", "API-KEY=xyz", "API-KEY=" + Placeholder},
		{"access key", "ACCESS_KEY=xyz", "ACCESS_KEY=" + Placeholder},
		{"connection string", "CONNECTION_STRING=Server=x;Pwd=y", "CONNECTION_STRING=" + Placeholder},
		{"token case insensitive", "authToken=abc", "authToken=" + Placeholder},
		{"empty value still redacted", "PASSWORD=", "PASSWORD=" + Placeholder},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, Value(tc.input))
		})
	}
}

func TestValue_DoesNotRedactBenignKeyValuePairs(t *testing.T) {
	cases := []string{
		"SIGNAL_FOLDER=/var/log/azure/events",
		"VERBOSE_LOG_FILE_FULL_PATH=/var/log/azure/vmwatch.log",
		"REGION=eastus",
		"COUNT=5",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			require.Equal(t, in, Value(in))
		})
	}
}

func TestValue_StripsUrlQueryString(t *testing.T) {
	in := "https://acct.blob.core.windows.net/c/cfg.json?sv=2021&sig=SECRETSIG&se=2030"
	want := "https://acct.blob.core.windows.net/c/cfg.json?" + Placeholder
	require.Equal(t, want, Value(in))
}

func TestValue_StripsUrlQueryInBenignKeyValue(t *testing.T) {
	// Key is not sensitive, but the URL value carries a SAS token in the query.
	in := "GLOBAL_CONFIG_URL=https://acct.blob.core.windows.net/c/cfg.json?sig=SECRET"
	want := "GLOBAL_CONFIG_URL=https://acct.blob.core.windows.net/c/cfg.json?" + Placeholder
	require.Equal(t, want, Value(in))
}

func TestValue_UrlWithoutQueryUnchanged(t *testing.T) {
	in := "https://acct.blob.core.windows.net/c/cfg.json"
	require.Equal(t, in, Value(in))
}

func TestValue_PlainStringUnchanged(t *testing.T) {
	in := "--global-config-url"
	require.Equal(t, in, Value(in))
}

func TestValue_LeadingEqualsNotTreatedAsKey(t *testing.T) {
	// idx must be > 0; a leading '=' has no key name to inspect.
	in := "=value"
	require.Equal(t, in, Value(in))
}

func TestSlice_RedactsEachElementAndCopies(t *testing.T) {
	in := []string{
		"--global-config-url",
		"https://acct.blob.core.windows.net/c/cfg.json?sig=SECRET",
		"--setenv",
		"AZ_STORAGE_SAS_TOKEN_BASE64=abc123",
		"SIGNAL_FOLDER=/var/log/azure/events",
	}
	want := []string{
		"--global-config-url",
		"https://acct.blob.core.windows.net/c/cfg.json?" + Placeholder,
		"--setenv",
		"AZ_STORAGE_SAS_TOKEN_BASE64=" + Placeholder,
		"SIGNAL_FOLDER=/var/log/azure/events",
	}
	got := Slice(in)
	require.Equal(t, want, got)

	// The original slice must be left untouched.
	require.Equal(t, "https://acct.blob.core.windows.net/c/cfg.json?sig=SECRET", in[1])
	require.Equal(t, "AZ_STORAGE_SAS_TOKEN_BASE64=abc123", in[3])
}

func TestSlice_EmptyAndNil(t *testing.T) {
	require.Equal(t, []string{}, Slice(nil))
	require.Equal(t, []string{}, Slice([]string{}))
}

func TestText_StripsSasFromJsonKeepingQuotesIntact(t *testing.T) {
	in := "\t\"globalConfigUrl\": \"https://acct.blob.core.windows.net/c/cfg.json?sv=2025-11-05&sig=Te8r%2FkH2%3D\"\n"
	want := "\t\"globalConfigUrl\": \"https://acct.blob.core.windows.net/c/cfg.json?" + Placeholder + "\"\n"
	require.Equal(t, want, Text(in))
}

func TestText_StripsMultipleUrlsInMultilineText(t *testing.T) {
	in := "line1 https://a.example.com/x?sig=AAA more\nline2 http://b.example.com/y?token=BBB end"
	want := "line1 https://a.example.com/x?" + Placeholder + " more\nline2 http://b.example.com/y?" + Placeholder + " end"
	require.Equal(t, want, Text(in))
}

func TestText_DoesNotApplyKeyValueRedaction(t *testing.T) {
	// Unlike Value, Text must not redact KEY=VALUE pairs (no first-'=' heuristic),
	// so unrelated content is preserved verbatim.
	in := "AZ_STORAGE_SAS_TOKEN_BASE64=abc123 no-url-here"
	require.Equal(t, in, Text(in))
}

func TestText_NoUrlUnchanged(t *testing.T) {
	in := "just some log output with an = sign and no urls"
	require.Equal(t, in, Text(in))
}

func TestValue_StripsUrlQueryInJsonKeepsClosingQuote(t *testing.T) {
	in := "\"https://acct.blob.core.windows.net/c/cfg.json?sig=SECRET\""
	want := "\"https://acct.blob.core.windows.net/c/cfg.json?" + Placeholder + "\""
	require.Equal(t, want, Value(in))
}

func TestJSON_RedactsSensitiveParameterOverrides(t *testing.T) {
	in := "{\n\t\"parameterOverrides\": {\n\t\t\"AZ_STORAGE_SAS_TOKEN_BASE64\": \"abc123\",\n\t\t\"EVENT_HUB_OUTPUT_NAME\": \"evhname\"\n\t}\n}"
	got := JSON(in)
	require.Contains(t, got, "\"AZ_STORAGE_SAS_TOKEN_BASE64\": \""+Placeholder+"\"")
	require.Contains(t, got, "\"EVENT_HUB_OUTPUT_NAME\": \"evhname\"")
	require.NotContains(t, got, "abc123")
}

func TestJSON_StripsSasFromGlobalConfigUrlAndStaysValid(t *testing.T) {
	type settings struct {
		GlobalConfigUrl string `json:"globalConfigUrl"`
		Password        string `json:"password"`
		AccountKey      string `json:"storageAccountKey"`
		Region          string `json:"region"`
		Count           int    `json:"count"`
	}
	raw, err := json.MarshalIndent(settings{
		GlobalConfigUrl: "https://acct.blob.core.windows.net/c/cfg.json?sv=2025-11-05&sig=Te8r%2FkH2%3D",
		Password:        "hunter2",
		AccountKey:      "AAAABBBBCCCC==",
		Region:          "eastus",
		Count:           5,
	}, "", "\t")
	require.NoError(t, err)

	got := JSON(string(raw))

	// Output must remain valid JSON.
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(got), &out))

	require.Equal(t, "https://acct.blob.core.windows.net/c/cfg.json?"+Placeholder, out["globalConfigUrl"])
	require.Equal(t, Placeholder, out["password"])
	require.Equal(t, Placeholder, out["storageAccountKey"])
	require.Equal(t, "eastus", out["region"])
	require.Equal(t, float64(5), out["count"])
	require.NotContains(t, got, "hunter2")
	require.NotContains(t, got, "AAAABBBBCCCC")
	require.NotContains(t, got, "sig=")
}

func TestJSON_LeavesNonStringSensitiveValuesUntouched(t *testing.T) {
	// Sensitive-looking keys with non-string values must not be corrupted.
	in := "{\n\t\"useToken\": true,\n\t\"tokenCount\": 3\n}"
	require.Equal(t, in, JSON(in))
}

func TestJSON_NoSensitiveFieldsUnchanged(t *testing.T) {
	in := "{\n\t\"region\": \"eastus\",\n\t\"port\": 8080\n}"
	require.Equal(t, in, JSON(in))
}
