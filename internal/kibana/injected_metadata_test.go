// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// sampleLoginPage represents Kibana login page without redundant styles, fonts, noise in metadata, etc.
var sampleLoginPage = []byte(`<!DOCTYPE html><html lang="en"><head><meta charSet="utf-8"/><meta http-equiv="X-UA-Compatible" content="IE=edge,chrome=1"/><meta name="viewport" content="width=device-width"/><title>Elastic</title><style>
          @keyframes kbnProgress {
            0% {
              transform: scaleX(1) translateX(-100%);
            }

            100% {
              transform: scaleX(1) translateX(100%);
            }
          }
        </style><link rel="stylesheet" type="text/css" href="/44185/bundles/kbn-ui-shared-deps/kbn-ui-shared-deps.css"/><link rel="stylesheet" type="text/css" href="/44185/bundles/kbn-ui-shared-deps/kbn-ui-shared-deps.v8.light.css"/><link rel="stylesheet" type="text/css" href="/node_modules/@kbn/ui-framework/dist/kui_light.css"/><link rel="stylesheet" type="text/css" href="/ui/legacy_light_theme.css"/><meta name="add-styles-here"/><meta name="add-scripts-here"/></head><body><kbn-csp data="{&quot;strictCsp&quot;:false}"></kbn-csp><kbn-injected-metadata data="{&quot;version&quot;:&quot;7.15.1&quot;,&quot;buildNumber&quot;:44185,&quot;branch&quot;:&quot;7.15&quot;,&quot;basePath&quot;:&quot;&quot;}"></kbn-injected-metadata><div class="kbnWelcomeView" id="kbn_loading_message" style="display:none" data-test-subj="kbnLoadingMessage"><div class="kbnLoaderWrap"><h2 class="kbnWelcomeTitle">Please upgrade your browser</h2><div class="kbnWelcomeText">This Elastic installation has strict security requirements enabled that your current browser does not meet.</div></div><script>
            // Since this is an unsafe inline script, this code will not run
            // in browsers that support content security policy(CSP). This is
            // intentional as we check for the existence of __kbnCspNotEnforced__ in
            // bootstrap.
            window.__kbnCspNotEnforced__ = true;
          </script><script src="/bootstrap.js"></script></body></html>`)

func TestExtractRawInjectedMetadata(t *testing.T) {
	im, err := extractInjectedMetadata(sampleLoginPage)
	require.NoError(t, err)
	require.Equal(t, "7.15.1", im.Version)
}
