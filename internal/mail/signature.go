package mail

import "fmt"

func GetSignature(sendAsEmail string) (string, error) {
	if sendAsEmail == "default" {
		resp, err := Service.Users.Settings.SendAs.List("me").Do()
		if err != nil {
			return "", HandleError(err)
		}
		for _, sa := range resp.SendAs {
			if sa.IsDefault {
				return sa.Signature, nil
			}
		}
		for _, sa := range resp.SendAs {
			if sa.IsPrimary {
				return sa.Signature, nil
			}
		}
		return "", fmt.Errorf("no default signature found")
	}

	sa, err := Service.Users.Settings.SendAs.Get("me", sendAsEmail).Do()
	if err != nil {
		return "", HandleError(err)
	}
	return sa.Signature, nil
}
