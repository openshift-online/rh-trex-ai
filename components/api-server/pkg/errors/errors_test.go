package errors

import (
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
)

func TestErrorFormatting(t *testing.T) {
	RegisterTestingT(t)
	err := New(ErrorGeneral, "test %s, %d", "errors", 1)
	Expect(err.Reason).To(Equal("test errors, 1"))
}

func TestErrorFind(t *testing.T) {
	RegisterTestingT(t)
	exists, err := Find(ErrorNotFound)
	Expect(exists).To(Equal(true))
	Expect(err.Code).To(Equal(ErrorNotFound))

	// Hopefully we never reach 91,823,719 error codes or this test will fail
	exists, err = Find(ServiceErrorCode(91823719))
	Expect(exists).To(Equal(false))
	Expect(err).To(BeNil())
}

func TestUnprocessableEntity(t *testing.T) {
	RegisterTestingT(t)
	err := UnprocessableEntity("missing %s", "repository_id")
	Expect(err.Code).To(Equal(ErrorUnprocessableEntity))
	Expect(err.HttpCode).To(Equal(http.StatusUnprocessableEntity))
	Expect(err.Reason).To(Equal("missing repository_id"))
}
