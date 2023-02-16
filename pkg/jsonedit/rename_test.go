package jsonedit

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Rename json field", func() {
	When("Renaming a simple field", func() {
		It("is renamed successfully", func() {
			input := []byte(`{"foobar":"myvalue"}`)
			fields := map[string]string{
				"foobar": "barfoo",
			}
			output, err := Rename(input, fields)
			Expect(err).NotTo(HaveOccurred())

			var out map[string]string
			err = json.Unmarshal(output, &out)
			Expect(err).NotTo(HaveOccurred())
			_, ok := out["foobar"]
			Expect(ok).To(BeFalse())

			val, ok := out["barfoo"]
			Expect(ok).To(BeTrue())
			Expect(val).To(Equal("myvalue"))
		})
	})
	When("Renaming multiple simple fields", func() {
		It("are renamed successfully", func() {
			input := []byte(`{"foobar":"myvalue","whatever":"example"}`)
			fields := map[string]string{
				"foobar":   "barfoo",
				"whatever": "we",
			}
			output, err := Rename(input, fields)
			Expect(err).NotTo(HaveOccurred())

			var out map[string]string
			err = json.Unmarshal(output, &out)
			Expect(err).NotTo(HaveOccurred())
			_, ok := out["foobar"]
			Expect(ok).To(BeFalse())

			val, ok := out["barfoo"]
			Expect(ok).To(BeTrue())
			Expect(val).To(Equal("myvalue"))

			_, ok = out["whatever"]
			Expect(ok).To(BeFalse())
			val, ok = out["we"]
			Expect(ok).To(BeTrue())
			Expect(val).To(Equal("example"))
		})
	})

	When("Renaming deep fields", func() {
		It("are renamed successfully", func() {
			input := []byte(`{"foobar":"myvalue","whatever":"example","foo":{"bar":"qux"}}`)
			fields := map[string]string{
				"foo.bar": "foo.baz",
			}
			output, err := Rename(input, fields)
			Expect(err).NotTo(HaveOccurred())

			var out map[string]interface{}
			err = json.Unmarshal(output, &out)
			Expect(err).NotTo(HaveOccurred())
			nested, ok := out["foo"]
			Expect(ok).To(BeTrue())

			foo, ok := nested.(map[string]interface{})
			Expect(ok).To(BeTrue())
			_, ok = foo["bar"]
			Expect(ok).To(BeFalse())
			baz, ok := foo["baz"]
			Expect(ok).To(BeTrue())
			Expect(baz).To(Equal("qux"))
		})
	})

	When("Renaming unexsting fields", func() {
		It("are ignored successfully", func() {
			input := []byte(`{"foobar":"myvalue","whatever":"example","foo":{"bar":"qux"}}`)
			fields := map[string]string{
				"unexsting.field": "whatever.i.please",
			}
			output, err := Rename(input, fields)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal(input))
		})
	})

	When("Renaming all elements in a list", func() {
		It("renamed successfully", func() {
			input := []byte(`{"foobar":"myvalue","whatever":"example","foo":[{"bar":"qux"},{"bar":"fuzz"},{"bar":"rab"}]}`)
			fields := map[string]string{
				"foo[*].bar": "foo[*].foobar",
			}
			output, err := Rename(input, fields)
			Expect(err).NotTo(HaveOccurred())

			var out map[string]interface{}
			err = json.Unmarshal(output, &out)
			Expect(err).NotTo(HaveOccurred())

			foo, ok := out["foo"]
			Expect(ok).To(BeTrue())

			list, ok := foo.([]interface{})
			Expect(ok).To(BeTrue())

			Expect(len(list)).To(Equal(3))
			for _, i := range list {
				item, ok := i.(map[string]interface{})
				Expect(ok).To(BeTrue())
				_, ok = item["bar"]
				Expect(ok).To(BeFalse())

				_, ok = item["foobar"]
				Expect(ok).To(BeTrue())
			}
		})
	})

	When("Rename an empty list field", func() {
		It("should return empty list", func() {
			input := []byte(`{"foobar":"myvalue"}`)
			paths := map[string]string{
				"foo.bar[*].age":      "foo.bar[*].oldness",
				"foo.bar[*].qux.fuzz": "foo.bar[*].qux.fuz",
			}
			output, err := Rename(input, paths)
			Expect(err).NotTo(HaveOccurred())

			var out map[string]interface{}
			err = json.Unmarshal(output, &out)
			Expect(err).NotTo(HaveOccurred())

			_, ok := out["foo"]
			Expect(ok).To(BeFalse())
		})
	})
})
