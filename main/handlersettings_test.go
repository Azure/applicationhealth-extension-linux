package main

import "testing"
import "github.com/stretchr/testify/require"

func Test_handlerSettingsValidate(t *testing.T) {
    // tcp includes request path
    require.Equal(t, errTcpMustNotIncludeRequestPath, handlerSettings{
        publicSettings{Protocol: "tcp", Port: 80, RequestPath: "RequestPath"},
        protectedSettings{},
    }.validate())

    // tcp without port
    require.Equal(t, errTcpConfigurationMustIncludePort, handlerSettings{
        publicSettings{Protocol: "tcp"},
        protectedSettings{},
    }.validate())

    // probe settle time should be less than 120 seconds
    require.Equal(t, errProbeSettleTimeExceedsThreshold, handlerSettings{
        publicSettings{Protocol: "http", IntervalInSeconds: 60, NumberOfProbes: 3},
        protectedSettings{},
    }.validate())

    require.Nil(t, handlerSettings{
        publicSettings{Protocol: "tcp", Port: 80},
        protectedSettings{},
    }.validate())

    require.Nil(t, handlerSettings{
        publicSettings{Protocol: "http", RequestPath: "healthEndpoint"},
        protectedSettings{},
    }.validate())

    require.Nil(t, handlerSettings{
        publicSettings{Protocol: "https", RequestPath: "healthEndpoint"},
        protectedSettings{},
    }.validate())
	
    require.Nil(t, handlerSettings{
        publicSettings{Protocol: "https", IntervalInSeconds: 30, NumberOfProbes: 3},
        protectedSettings{},
    }.validate())
}

func Test_toJSON_empty(t *testing.T) {
	s, err := toJSON(nil)
	require.Nil(t, err)
	require.Equal(t, "{}", s)
}

func Test_toJSON(t *testing.T) {
	s, err := toJSON(map[string]interface{}{
		"a": 3})
	require.Nil(t, err)
	require.Equal(t, `{"a":3}`, s)
}
