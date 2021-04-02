package ws

import (
	"reflect"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	input := []byte("To be encrypted content.")
	type args struct {
		key   []byte
		input []byte
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "Test case 1",
			args: args{
				key:   cipherKey,
				input: input,
			},
			want:    []byte(""),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Encrypt(tt.args.key, tt.args.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encrypt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			output, err := Decrypt(tt.args.key, got)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encrypt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(input, output) {
				t.Errorf("Encrypt() = %v, want %v", got, tt.want)
			}
		})
	}
}
