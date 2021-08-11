package main

import (
	"crypto/x509"
	"fmt"
	"net/http"

	"io/ioutil"

	"encoding/base64"
	"encoding/xml"

	saml2 "github.com/russellhaering/gosaml2"
	"github.com/russellhaering/gosaml2/types"
	dsig "github.com/russellhaering/goxmldsig"
)

func main() {
	res, err := http.Get("https://app-qa.t.brazenconnect.com/sso/saml/metadata")
	if err != nil {
		panic(err)
	}

	rawMetadata, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}

	metadata := &types.EntityDescriptor{}
	err = xml.Unmarshal(rawMetadata, metadata)
	if err != nil {
		panic(err)
	}

	certStore := dsig.MemoryX509CertificateStore{
		Roots: []*x509.Certificate{},
	}



	xcert := "MIIDNzCCAh8CAQEwDQYJKoZIhvcNAQEFBQAwZDELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAk1BMRowGAYDVQQKDBFNb25zdGVyIFdvcmxkd2lkZTENMAsGA1UECwwEVEVTVDEdMBsGA1UEAwwUVEVTVC1RQS1DQS0yMDExLjA2LTIwHhcNMTMwODE1MTYzMzMxWhcNMjMwODEzMTYzMzMxWjBfMQswCQYDVQQGEwJVUzELMAkGA1UECAwCTUExGjAYBgNVBAoMEU1vbnN0ZXIgV29ybGR3aWRlMQ0wCwYDVQQLDARURVNUMRgwFgYDVQQDDA9RQS1TQU1MLTIwMTMuMDcwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDgGQ4aEfFXb0KoFbo3Xt1vbGO7GS41Ut6lKJ5ZFaYxBXitz8pOiXd0rcjnCXG3gY6AYnY4Ex2eXZVe0Pe9M3ynefuwi16YZBgQ4DH8r/x7SDPwPVETjrYguwmjyo0Q8iY5XRyyHWexf5gHc3P+WUdJZGKqhCginokdVC/sKHwvGZYX+c18DG4kTL+wM7KnZ9E01L31nlnQTuIM+d5XMnwmHRj19DGJ8BsDk4pcVAx50smBYP3gSkhLzfb67r1GH9M1N+M9BxfhgODhkyiwNcAm/05I6QHCOws1EYHe5tOYVHBg6UDbMulZEOSjW6Em4hj1LrMN4/T0ke1/7SSVtqNTAgMBAAEwDQYJKoZIhvcNAQEFBQADggEBAB3fxgKHp7ruSllVuoUWSsyWf2Al1WlaJhqMro26De+7wQBED8Ron911w1JzhMrlUtz9GY16mR0q7Abe3D+8m/9wJ17sADIdp7DcbPUSeYHp9r2Lvadbmmx6bXkP5kXgQu9P8OibeFGUKhb2sZ42fEVvtqHH+XtRDqGM+PamfV2DX13eajcBAAQjgc+hz0tVgGtA8rSP6ec98jW6HUaC9+CQDJ71/s+DVnRp1by8J/EC6Es0/KbYzwfRlVssvhK1OiUSuED2T0uSancYNIhx2WXmtDQV3q1K/ffMs7rEXeqrgLHE0RW6gbNyeQduifro29QFMIwYbFihEgjxCsSko8I="
	if certData, err := base64.StdEncoding.DecodeString(xcert); err == nil {
		if idpCert, err := x509.ParseCertificate(certData); err == nil {
			certStore.Roots = append(certStore.Roots, idpCert)
		}
	}


	// We sign the AuthnRequest with a random key because Okta doesn't seem
	// to verify these.
	randomKeyStore := dsig.RandomKeyStoreForTest()

	sp := &saml2.SAMLServiceProvider{
		IdentityProviderSSOURL:      metadata.IDPSSODescriptor.SingleSignOnServices[0].Location,
		IdentityProviderIssuer:      metadata.EntityID,
		ServiceProviderIssuer:       "https://app-qa.t.brazenconnect.com/sso/saml/login",
		AssertionConsumerServiceURL: "https://brazen-webhook.herokuapp.com/saml_callback",
		SignAuthnRequests:           true,
		AudienceURI:                 "https://app-qa.t.brazenconnect.com/sso/saml/login",
		IDPCertificateStore:         &certStore,
		SPKeyStore:                  randomKeyStore,
	}

	http.HandleFunc("/register/1", serveFiles)

	http.HandleFunc("/saml_callback", func(rw http.ResponseWriter, req *http.Request) {
		err := req.ParseForm()
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		assertionInfo, err := sp.RetrieveAssertionInfo(req.FormValue("SAMLResponse"))
		if err != nil {
			rw.WriteHeader(http.StatusForbidden)
			return
		}

		if assertionInfo.WarningInfo.InvalidTime {
			rw.WriteHeader(http.StatusForbidden)
			return
		}

		if assertionInfo.WarningInfo.NotInAudience {
			rw.WriteHeader(http.StatusForbidden)
			return
		}

		fmt.Fprintf(rw, "NameID: %s\n", assertionInfo.NameID)

		fmt.Fprintf(rw, "Assertions:\n")

		for key, val := range assertionInfo.Values {
			fmt.Fprintf(rw, "  %s: %+v\n", key, val)
		}

		fmt.Fprintf(rw, "\n")

		fmt.Fprintf(rw, "Warnings:\n")
		fmt.Fprintf(rw, "%+v\n", assertionInfo.WarningInfo)
	})

	println("Visit this URL To Authenticate:")
	authURL, err := sp.BuildAuthURL("https://brazen-webhook.herokuapp.com/register/1")
	if err != nil {
		panic(err)
	}

	println(authURL)

	println("Supply:")
	fmt.Printf("  SP ACS URL      : %s\n", sp.AssertionConsumerServiceURL)

	err = http.ListenAndServe(":8800", nil)
	if err != nil {
		panic(err)
	}
}


func serveFiles(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
	p := "." + r.URL.Path
	if p == "./register/1" {
		p = "./1.html"
	}
	http.ServeFile(w, r, p)
}
