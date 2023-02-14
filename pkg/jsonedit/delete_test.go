package jsonedit

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Delete json field", func() {
	When("Delete a simple field", func() {
		It("is deleted successfully", func() {
			input := []byte(`{"foobar":"myvalue"}`)
			paths := []string{
				"foobar",
			}
			output, err := Delete(input, paths)
			Expect(err).NotTo(HaveOccurred())

			Expect(output).To(Equal([]byte("{}")))
		})
	})
	When("Delete field in a list", func() {
		It("deletes successfully", func() {
			input := []byte(`{"foobar":"myvalue","foo":{"bar":[{"colour":"red","fruit":"apple"},{"colour":"yellow","fruit":"banana"},{"colour":"green","fruit":"kiwi"}]}}`)
			paths := []string{
				"foo.bar[*].colour",
			}
			output, err := Delete(input, paths)
			Expect(err).NotTo(HaveOccurred())

			var out map[string]interface{}
			err = json.Unmarshal(output, &out)
			Expect(err).NotTo(HaveOccurred())

			f, ok := out["foo"]
			Expect(ok).To(BeTrue())

			foo, ok := f.(map[string]interface{})
			Expect(ok).To(BeTrue())

			b, ok := foo["bar"]
			Expect(ok).To(BeTrue())
			bar, ok := b.([]interface{})
			Expect(ok).To(BeTrue())

			for _, i := range bar {
				item, ok := i.(map[string]interface{})
				Expect(ok).To(BeTrue())

				_, ok = item["colour"]
				Expect(ok).To(BeFalse())
				_, ok = item["fruit"]
				Expect(ok).To(BeTrue())

			}
		})
	})
})
