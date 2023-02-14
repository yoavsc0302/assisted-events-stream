package jsonedit

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Transform json field", func() {
	When("Transform a simple field", func() {
		It("is transformed successfully", func() {
			input := []byte(`{"foobar":"myvalue"}`)
			paths := []string{
				"foobar",
			}
			transformFn := func(unpacked interface{}) (interface{}, error) {
				if val, ok := unpacked.(string); ok {
					return "foobar" + val, nil
				}
				return unpacked, nil
			}
			output, err := Transform(input, paths, transformFn)
			Expect(err).NotTo(HaveOccurred())

			var out map[string]string
			err = json.Unmarshal(output, &out)
			Expect(err).NotTo(HaveOccurred())
			val, ok := out["foobar"]
			Expect(ok).To(BeTrue())
			Expect(val).To(Equal("foobarmyvalue"))
		})
	})
	When("Transform a list field", func() {
		It("are transformed successfully", func() {
			input := []byte(`{"foobar":"myvalue","foo":{"bar":[{"age":12},{"age":21},{"age":22},{"age":99}]}}`)
			paths := []string{
				"foo.bar[*].age",
			}
			transformFn := func(unpacked interface{}) (interface{}, error) {
				return 1, nil
			}
			output, err := Transform(input, paths, transformFn)
			Expect(err).NotTo(HaveOccurred())

			var out map[string]interface{}
			err = json.Unmarshal(output, &out)
			Expect(err).NotTo(HaveOccurred())
			foo, ok := out["foo"]
			Expect(ok).To(BeTrue())
			f, ok := foo.(map[string]interface{})
			Expect(ok).To(BeTrue())
			l, ok := f["bar"]
			Expect(ok).To(BeTrue())

			list, ok := l.([]interface{})
			Expect(ok).To(BeTrue())

			for _, i := range list {
				item, ok := i.(map[string]interface{})
				Expect(ok).To(BeTrue())
				age, ok := item["age"]
				Expect(ok).To(BeTrue())
				Expect(age).To(Equal(float64(1)))
			}
		})
	})
})
