package pollinghandler

import (
	"fmt"
	db "simplemts/lib/database"

	"github.com/go-mail/mail"
)

func (ph *PollingHandler) clearEmailSub(isIncoming bool) (err error) {
	type referral struct {
		Id             int               `json:"Id" validate:"required"`
		ReferralStatus db.ReferralStatus `json:"ReferralStatus" validate:"required"`
		db.PatientObject
		Origin      string `json:"Origin" validate:"required"`
		Destination string `json:"Destination" validate:"required"`
		Reason      string `json:"Reason" validate:"required"`
		Created     int64  `json:"Created" validate:"required"`
	}
	response := struct {
		Referrals []referral `json:"referrals" validate:"required,dive"`
	}{}
	url := "/incoming"
	if !isIncoming {
		url = "/outgoing"
	}
	err = ph.requestDecode(url, 200, &response)
	if err != nil {
		return fmt.Errorf("Could not get referrals: %s", err)
	}
	for _, ref := range response.Referrals {
		if ref.ReferralStatus == db.Complete {
			ph.staffCompleteEmail[ref.Id] = true
			ph.docCompleteEmail[ref.Id] = true
		}
		if ref.ReferralStatus == db.Consented {
			ph.staffGrantEmail[ref.Id] = true
		}
		if ref.ReferralStatus == db.NotGranted {
			ph.docNotGrantEmail[ref.Id] = true
		}
	}
	return nil
}

func (ph *PollingHandler) clearEmail() (err error) {
	// incoming
	err = ph.clearEmailSub(true)
	if err != nil {
		return
	}
	// outgoing
	err = ph.clearEmailSub(false)
	if err != nil {
		return
	}
	return nil
}

func emailDocComplete(doctorName string, patientName string, referralId int, date string, hospitalName string) {
	m := mail.NewMessage()
	m.SetHeader("From", "email@redacted")
	m.SetHeader("To", "email2@redacted")
	m.SetHeader("Subject", fmt.Sprintf("[Patient Referral System] Referral Complete (ID:%d)", referralId))
	m.SetBody("text/plain", fmt.Sprintf(
		"Referral ID:%d\n\nDear %s,\nThis is an update on %s's referral on %s to %s.\n\nThe referral process is complete, please check details in the referral system",
		referralId, doctorName, patientName, date, hospitalName))
	d := mail.NewDialer("smtp.gmail.com", 587, "email@redacted", "csiu giwt cxwj eflo")
	if err := d.DialAndSend(m); err != nil {

		panic(err)

	}
	fmt.Println("Email Successfully Sent")
}

func emailDocNotGrant(doctorName string, patientName string, referralId int, date string, hospitalName string) {
	m := mail.NewMessage()
	m.SetHeader("From", "email@redacted")
	m.SetHeader("To", "email2@redacted")
	m.SetHeader("Subject", fmt.Sprintf("[Patient Referral System] Referral Permission Denied (ID:%d)", referralId))
	m.SetBody("text/plain", fmt.Sprintf(
		"Referral ID:%d\n\nDear %s,\nThis is an update on %s's referral on %s to %s.\n\n %s has denied the request to refer the patient, please check details in the referral system",
		referralId, doctorName, patientName, date, hospitalName, hospitalName))
	d := mail.NewDialer("smtp.gmail.com", 587, "email@redacted", "csiu giwt cxwj eflo")
	if err := d.DialAndSend(m); err != nil {

		panic(err)

	}
	fmt.Println("Email Successfully Sent")
}

func emailStaffComplete(referralId int, date string, destinationHospital string, originHospital string) {
	m := mail.NewMessage()
	m.SetHeader("From", "email@redacted")
	m.SetHeader("To", "email2@redacted")
	m.SetHeader("Subject", fmt.Sprintf("[Patient Referral System] Referral from %s Complete (ID:%d)", originHospital, referralId))
	m.SetBody("text/plain", fmt.Sprintf(
		"Referral ID:%d\n\nDear %s staff,\nThis is an update on a referral from %s to your hospital, made on %s.\n\nThe referral process is complete, please check details in the referral system.",
		referralId, destinationHospital, originHospital, date))
	d := mail.NewDialer("smtp.gmail.com", 587, "email@redacted", "csiu giwt cxwj eflo")
	if err := d.DialAndSend(m); err != nil {

		panic(err)

	}
	fmt.Println("Email Successfully Sent")
}

func emailStaffGrant(referralId int, date string, destinationHospital string, originHospital string) {
	m := mail.NewMessage()
	m.SetHeader("From", "email@redacted")
	m.SetHeader("To", "email2@redacted")
	m.SetHeader("Subject", fmt.Sprintf("[Patient Referral System] Requesting to Refer patient from %s (ID:%d)", originHospital, referralId))
	m.SetBody("text/plain", fmt.Sprintf(
		"Referral ID:%d\n\nDear %s staff,\nThis is a request to referral a patient from %s to your hospital, made on %s.\n\nPlease check request details in the referral system.",
		referralId, destinationHospital, originHospital, date))
	d := mail.NewDialer("smtp.gmail.com", 587, "email@redacted", "csiu giwt cxwj eflo")
	if err := d.DialAndSend(m); err != nil {

		panic(err)

	}
	fmt.Println("Email Successfully Sent")
}
