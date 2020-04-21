package schema

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/dgraph/testutil"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
)

const complexSchema = `schema {
	query: TestType
  }

  input TestInputObject {
	a: String = test
	b: [String]
	c: String = null
  }


  input TestType {
	complex: TestInputObject
  }
  `

func TestIntrospectionQuery(t *testing.T) {
	simpleSchema := `schema {
		query: QueryRoot
	}

	type QueryRoot {
		onlyField: String
	}`

	deprecatedSchema := `
	type TestDeprecatedObject {
		dep: String @deprecated
		depReason: String @deprecated(reason: "because")
		notDep: String
	}

	enum TestDeprecatedEnum {
		dep @deprecated
		depReason @deprecated(reason: "because")
		notDep
	}
	`

	iprefix := "testdata/introspection/input"
	oprefix := "testdata/introspection/output"

	var tests = []struct {
		name       string
		schema     string
		queryFile  string
		outputFile string
	}{
		{
			"Filter on __type",
			simpleSchema,
			filepath.Join(iprefix, "type_filter.txt"),
			filepath.Join(oprefix, "type_filter.json"),
		},
		{"Filter __Schema on __type",
			simpleSchema,
			filepath.Join(iprefix, "type_schema_filter.txt"),
			filepath.Join(oprefix, "type_schema_filter.json"),
		},
		{"Filter object type __type",
			simpleSchema,
			filepath.Join(iprefix, "type_object_name_filter.txt"),
			filepath.Join(oprefix, "type_object_name_filter.json"),
		},
		{"Filter complex object type __type",
			complexSchema,
			filepath.Join(iprefix, "type_complex_object_name_filter.txt"),
			filepath.Join(oprefix, "type_complex_object_name_filter.json"),
		},
		{"Deprecated directive on type with deprecated",
			simpleSchema + deprecatedSchema,
			filepath.Join(iprefix, "type_withdeprecated.txt"),
			filepath.Join(oprefix, "type_withdeprecated.json"),
		},
		{"Deprecated directive on type without deprecated",
			simpleSchema + deprecatedSchema,
			filepath.Join(iprefix, "type_withoutdeprecated.txt"),
			filepath.Join(oprefix, "type_withoutdeprecated.json"),
		},
		{"Deprecated directive on enum with deprecated",
			simpleSchema + deprecatedSchema,
			filepath.Join(iprefix, "enum_withdeprecated.txt"),
			filepath.Join(oprefix, "enum_withdeprecated.json"),
		},
		{"Deprecated directive on enum without deprecated",
			simpleSchema + deprecatedSchema,
			filepath.Join(iprefix, "enum_withoutdeprecated.txt"),
			filepath.Join(oprefix, "enum_withoutdeprecated.json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sch := gqlparser.MustLoadSchema(
				&ast.Source{Name: "schema.graphql", Input: tt.schema})

			q, err := ioutil.ReadFile(tt.queryFile)
			require.NoError(t, err)

			doc, gqlErr := parser.ParseQuery(&ast.Source{Input: string(q)})
			require.Nil(t, gqlErr)
			listErr := validator.Validate(sch, doc)
			require.Equal(t, 0, len(listErr))

			op := doc.Operations.ForName("")
			oper := &operation{op: op,
				vars:     map[string]interface{}{},
				query:    string(q),
				doc:      doc,
				inSchema: &schema{schema: sch},
			}
			require.NotNil(t, op)

			queries := oper.Queries()
			resp, err := Introspect(queries[0])
			require.NoError(t, err)

			expectedBuf, err := ioutil.ReadFile(tt.outputFile)
			require.NoError(t, err)
			testutil.CompareJSON(t, string(expectedBuf), string(resp))
		})
	}
}

func TestIntrospectionQueryMissingNameArg(t *testing.T) {
	sch := gqlparser.MustLoadSchema(
		&ast.Source{Name: "schema.graphql", Input: `
		schema {
			query: TestType
		}
	
		type TestType {
			testField: String
		}
	`})
	missingNameArgQuery := `
	{
    	__type {
	        name
    	}
	}`

	doc, gqlErr := parser.ParseQuery(&ast.Source{Input: missingNameArgQuery})
	require.Nil(t, gqlErr)

	listErr := validator.Validate(sch, doc)
	require.Equal(t, 1, len(listErr))
	require.Equal(t, "Field \"__type\" argument \"name\" of type \"String!\" is required but not provided.", listErr[0].Message)
}

func TestIntrospectionQueryWithVars(t *testing.T) {
	sch := gqlparser.MustLoadSchema(
		&ast.Source{Name: "schema.graphql", Input: complexSchema})

	q := `query filterNameOnType($name: String!) {
			__type(name: $name) {
				kind
				name
				inputFields {
					name
					type { ...TypeRef }
					defaultValue
				}
			}
		}

		fragment TypeRef on __Type {
			kind
			name
			ofType {
				kind
				name
				ofType {
					kind
					name
					ofType {
						kind
						name
					}
				}
			}
		}`

	doc, gqlErr := parser.ParseQuery(&ast.Source{Input: q})
	require.Nil(t, gqlErr)
	listErr := validator.Validate(sch, doc)
	require.Equal(t, 0, len(listErr))

	op := doc.Operations.ForName("")
	oper := &operation{op: op,
		vars:     map[string]interface{}{"name": "TestInputObject"},
		query:    q,
		doc:      doc,
		inSchema: &schema{schema: sch},
	}
	require.NotNil(t, op)

	queries := oper.Queries()
	resp, err := Introspect(queries[0])
	require.NoError(t, err)

	fname := "testdata/introspection/output/type_complex_object_name_filter.json"
	expectedBuf, err := ioutil.ReadFile(fname)
	require.NoError(t, err)
	testutil.CompareJSON(t, string(expectedBuf), string(resp))
}

func TestFullIntrospectionQuery(t *testing.T) {
	sch := gqlparser.MustLoadSchema(
		&ast.Source{Name: "schema.graphql", Input: `
	schema {
		query: TestType
	}

	type TestType {
		testField: String
	}
`})

	doc, gqlErr := parser.ParseQuery(&ast.Source{Input: introspectionQuery})
	require.Nil(t, gqlErr)

	listErr := validator.Validate(sch, doc)
	require.Equal(t, 0, len(listErr))

	op := doc.Operations.ForName("")
	require.NotNil(t, op)
	oper := &operation{op: op,
		vars:     map[string]interface{}{},
		query:    string(introspectionQuery),
		doc:      doc,
		inSchema: &schema{schema: sch},
	}

	queries := oper.Queries()
	resp, err := Introspect(queries[0])
	require.NoError(t, err)

	expectedBuf, err := ioutil.ReadFile("testdata/introspection/output/full_query.json")
	require.NoError(t, err)
	testutil.CompareJSON(t, string(expectedBuf), string(resp))
}

func Test(t *testing.T) {
	queryDoc, err := parser.ParseQuery(&ast.Source{Input: `mutation team(
$postID: Int){relatedUsers(id:$postID,id:$postID)@test}query{test()}`})
	fmt.Println(err)
	fmt.Println("**********Operations************")
	fmt.Println(queryDoc.Operations)
	fmt.Println(queryDoc.Operations[0].Name)
	fmt.Println(queryDoc.Operations[0].Directives)
	fmt.Println(queryDoc.Operations[0].Operation)
	fmt.Println(queryDoc.Operations[0].VariableDefinitions)
	fmt.Println(queryDoc.Operations[0].VariableDefinitions[0].Variable)
	fmt.Println()
	fmt.Println(queryDoc.Operations[1].Name)
	fmt.Println(queryDoc.Operations[1].Directives)
	fmt.Println(queryDoc.Operations[1].Operation)
	fmt.Println(queryDoc.Operations[1].VariableDefinitions)
	query := queryDoc.Operations[0].SelectionSet[0].(*ast.Field)
	fmt.Println("**********Query************")
	fmt.Println(query)
	fmt.Println(query.Alias)
	fmt.Println(query.SelectionSet)
	fmt.Println(query.Arguments)
	fmt.Println(query.Name)
	fmt.Println(query.Definition)
	fmt.Println(query.ObjectDefinition)
	fmt.Println(query.Directives)
	fmt.Println("**********Args************")
	fmt.Println(query.Arguments[0].Name, query.Arguments[0].Value)
}
