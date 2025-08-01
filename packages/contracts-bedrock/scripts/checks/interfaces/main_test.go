package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestGetContractDefinition(t *testing.T) {
	artifact := &Artifact{
		AST: ArtifactAST{
			Nodes: []ASTNode{
				{NodeType: "ContractDefinition", ContractDefinition: ContractDefinition{ContractKind: "interface", Name: "ITest"}},
				{NodeType: "ContractDefinition", ContractDefinition: ContractDefinition{ContractKind: "contract", Name: "Test"}},
				{NodeType: "ContractDefinition", ContractDefinition: ContractDefinition{ContractKind: "library", Name: "TestLib"}},
			},
		},
	}

	tests := []struct {
		name         string
		contractName string
		want         *ContractDefinition
	}{
		{"Find interface", "ITest", &ContractDefinition{ContractKind: "interface", Name: "ITest"}},
		{"Find contract", "Test", &ContractDefinition{ContractKind: "contract", Name: "Test"}},
		{"Find library", "TestLib", &ContractDefinition{ContractKind: "library", Name: "TestLib"}},
		{"Not found", "NonExistent", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getContractDefinition(artifact, tt.contractName)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getContractDefinition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetContractSemver(t *testing.T) {
	tests := []struct {
		name     string
		artifact *Artifact
		want     string
		wantErr  bool
	}{
		{
			name: "Valid semver",
			artifact: &Artifact{
				AST: ArtifactAST{
					Nodes: []ASTNode{
						{NodeType: "PragmaDirective", Literals: []string{"solidity", "^", "0.8.0"}},
					},
				},
			},
			want:    "solidity^0.8.0",
			wantErr: false,
		},
		{
			name: "Multiple pragmas",
			artifact: &Artifact{
				AST: ArtifactAST{
					Nodes: []ASTNode{
						{NodeType: "PragmaDirective", Literals: []string{"solidity", "^", "0.8.0"}},
						{NodeType: "PragmaDirective", Literals: []string{"abicoder", "v2"}},
					},
				},
			},
			want:    "solidity^0.8.0",
			wantErr: false,
		},
		{
			name: "No semver",
			artifact: &Artifact{
				AST: ArtifactAST{
					Nodes: []ASTNode{
						{NodeType: "ContractDefinition"},
					},
				},
			},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getContractSemver(tt.artifact)
			if (err != nil) != tt.wantErr {
				t.Errorf("getContractSemver() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getContractSemver() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeABI(t *testing.T) {
	tests := []struct {
		name string
		abi  string
		want string
	}{
		{
			name: "Replace interface types and add constructor",
			abi:  `[{"inputs":[{"internalType":"contract Test","name":"test","type":"address"}],"type":"function"}]`,
			want: `[{"inputs":[{"internalType":"contract ITest","name":"test","type":"address"}],"type":"function"},{"inputs":[],"stateMutability":"nonpayable","type":"constructor"}]`,
		},
		{
			name: "Convert __constructor__",
			abi:  `[{"type":"function","name":"__constructor__","inputs":[],"stateMutability":"nonpayable","outputs":[]}]`,
			want: `[{"type":"constructor","inputs":[],"stateMutability":"nonpayable"}]`,
		},
		{
			name: "Keep existing constructor",
			abi:  `[{"type":"constructor","inputs":[{"name":"param","type":"uint256"}]},{"type":"function","name":"test"}]`,
			want: `[{"type":"constructor","inputs":[{"name":"param","type":"uint256"}]},{"type":"function","name":"test"}]`,
		},
		{
			name: "Replace multiple interface types",
			abi:  `[{"inputs":[{"internalType":"contract Test1","name":"test1","type":"address"},{"internalType":"contract ITest2","name":"test2","type":"address"}],"type":"function"}]`,
			want: `[{"inputs":[{"internalType":"contract ITest1","name":"test1","type":"address"},{"internalType":"contract ITest2","name":"test2","type":"address"}],"type":"function"},{"inputs":[],"stateMutability":"nonpayable","type":"constructor"}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeABI(json.RawMessage(tt.abi))
			if err != nil {
				t.Errorf("normalizeABI() error = %v", err)
				return
			}
			var gotJSON, wantJSON interface{}
			if err := json.Unmarshal(got, &gotJSON); err != nil {
				t.Errorf("Error unmarshalling got JSON: %v", err)
				return
			}
			if err := json.Unmarshal([]byte(tt.want), &wantJSON); err != nil {
				t.Errorf("Error unmarshalling want JSON: %v", err)
				return
			}
			if !reflect.DeepEqual(gotJSON, wantJSON) {
				t.Errorf("normalizeABI() = %v, want %v", string(got), tt.want)
			}
		})
	}
}

func TestCompareABIs(t *testing.T) {
	tests := []struct {
		name string
		abi1 string
		abi2 string
		want bool
	}{
		{
			name: "Identical ABIs",
			abi1: `[{"type":"function","name":"test","inputs":[],"outputs":[]}]`,
			abi2: `[{"type":"function","name":"test","inputs":[],"outputs":[]}]`,
			want: true,
		},
		{
			name: "Different ABIs",
			abi1: `[{"type":"function","name":"test1","inputs":[],"outputs":[]}]`,
			abi2: `[{"type":"function","name":"test2","inputs":[],"outputs":[]}]`,
			want: false,
		},
		{
			name: "Different order, same content",
			abi1: `[{"type":"function","name":"test1","inputs":[],"outputs":[]},{"type":"function","name":"test2","inputs":[],"outputs":[]}]`,
			abi2: `[{"type":"function","name":"test2","inputs":[],"outputs":[]},{"type":"function","name":"test1","inputs":[],"outputs":[]}]`,
			want: true,
		},
		{
			name: "Different input types",
			abi1: `[{"type":"function","name":"test","inputs":[{"type":"uint256"}],"outputs":[]}]`,
			abi2: `[{"type":"function","name":"test","inputs":[{"type":"uint128"}],"outputs":[]}]`,
			want: false,
		},
		{
			name: "Different output types",
			abi1: `[{"type":"function","name":"test","inputs":[],"outputs":[{"type":"uint256"}]}]`,
			abi2: `[{"type":"function","name":"test","inputs":[],"outputs":[{"type":"uint128"}]}]`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := compareABIs(json.RawMessage(tt.abi1), json.RawMessage(tt.abi2))
			if err != nil {
				t.Errorf("compareABIs() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("compareABIs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckExclusion(t *testing.T) {
	// Fixed test list
	testExcludes := []string{
		"IERC20",
		"IEAS",
		"IERC721",
	}

	tests := []struct {
		name         string
		contractName string
		want         bool
	}{
		{"Excluded contract", "IERC20", true},
		{"Non-excluded contract", "IMyContract", false},
		{"Another excluded contract", "IEAS", true},
		{"Excluded contract (case-sensitive)", "ierc20", false},
		{"Excluded contract with prefix", "IERC20Extension", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkExclusion(tt.contractName, testExcludes); got != tt.want {
				t.Errorf("checkExclusion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeInternalType(t *testing.T) {
	tests := []struct {
		name         string
		internalType string
		want         string
	}{
		{"Replace contract X", "contract Test", "contract ITest"},
		{"Replace enum X", "enum MyEnum", "enum IMyEnum"},
		{"Replace struct I", "struct Whatever.MyStruct", "struct IWhatever.MyStruct"},
		{"Don't replace II", "contract IInternet", "contract IInternet"},
		{"No replacement needed", "uint256", "uint256"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeInternalType(tt.internalType); got != tt.want {
				t.Errorf("normalizeInternalType() = %v, want %v", got, tt.want)
			}
		})
	}
}
