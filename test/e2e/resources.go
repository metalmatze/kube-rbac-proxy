package e2e

import (
	"testing"

	"github.com/brancz/kube-rbac-proxy/test/kubetest"
)

func testResources(s *kubetest.Suite) kubetest.TestSuite {
	return func(t *testing.T) {
		kubetest.Scenario{
			Name: "Simple",

			Given: kubetest.Setups(
				kubetest.CreatedManifests(s.KubeConfig,
					"resources/serviceAccount.yaml",
					"resources/clusterRole.yaml",
					"resources/clusterRoleBinding.yaml",
					"resources/configMap.yaml",
					"resources/service.yaml",
					"resources/deployment.yaml",
				),
			),
			When: kubetest.Conditions(
				kubetest.PodsAreReady(s.KubeClient,
					1,
					"app=kube-rbac-proxy",
				),
			),
		}.Run(t)
	}
}
