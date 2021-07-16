#!/bin/bash

for s in "" Sticky; do
	for e in Big Little; do
		declare startP;
		declare order;
		if [ "$e" = "Big" ]; then
			startP=1;
			order=-1;
		else
			startP=0;
			order=1;
		fi;

		for rw in Read Write; do
			er="er";
			if [ "$rw" = "Write" ]; then
				er="r";
			fi;
			(
				cat <<HEREDOC
package byteio

// File automatically generated with ./gen.sh

import (
	"io"
	"math"
)

// ${s}${e}Endian${rw}${er} wraps a io.${rw}${er} to provide methods
// to make it easier to $rw fundamental types
type ${s}${e}Endian${rw}${er} struct {
	io.${rw}${er}
	buffer [9]byte
HEREDOC
				if [ ! -z "$s" ]; then
					echo "	Err    error";
					echo "	Count  int64";
				fi;

				echo "}";

				if [ ! -z "$s" ]; then
					cat <<HEREDOC

// ${rw} implements the io.${rw}${er} interface
func (e *${s}${e}Endian${rw}${er}) ${rw}(p []byte) (int, error) {
	if e.Err != nil {
		return 0, e.Err
	}
	var n int
	n, e.Err = e.${rw}${er}.$rw(p)
	e.Count += int64(n)
	return n, e.Err
}
HEREDOC
				fi;

				for t in "Int" "Uint" "Float"; do
					types="8 16 24 32 40 48 56 64"
					if [ "$t" = "Float" ]; then
						types="32 64";
					fi;
					for i in $types; do
						ti=$i;
						if [ $i -eq 24 ]; then
							ti=32;
						elif [ $i -gt 32 -a $i -lt 64 ]; then
							ti=64;
						fi;
						tu="$(echo "$t" | tr A-Z a-z)$ti";
						echo;
						echo "// ${rw}${t}${i} ${rw}s a $i bit $(echo "$t" | tr A-Z a-z) as a $tu using the underlying io.${rw}${er}";
						if [ ! -z "$s" ]; then
							echo "// Any errors and the running byte read count are stored instead or returned";
						fi;
						echo -n "func (e *${s}${e}Endian${rw}${er}) ${rw}${t}${i}(";
						if [ "$rw" = "Write" ]; then
							echo -n "d $tu) ";
							if [ -z "$s" ]; then
								echo -n "(";
							fi;
						else
							echo -n ") ";
							if [ -z "$s" ]; then
								echo -n "($tu, ";
							else
								echo -n "$tu ";
							fi;
						fi;
						if [ -z "$s" ]; then
							echo "int, error) {";
						else
							echo "{";
						fi;

						if [ ! -z "$s" ]; then
							echo "	if e.Err != nil {";
							echo -n "		return";
							if [ "$rw" = "Read" ]; then
								echo -n " 0";
							fi;
							echo;
							echo "	}";
						fi;
						if [ "$rw" = "Read" ]; then
							echo "	n, err := io.ReadFull(e.Reader, e.buffer[:$(( $i / 8 ))])";
							if [ ! -z "$s" ]; then
								echo "	e.Count += int64(n)";
							fi;
							echo "	if err != nil {";
							if [ -z "$s" ]; then
								echo "		return 0, n, err";
							else
								echo "		e.Err = err";
								echo "		return 0";
							fi;
					
							echo "	}";
							echo -n "	return ";

							if [ "$t" = "Int" ]; then
								echo -n "int$ti(";
							elif [ "$t" = "Float" ]; then
								echo -n "math.Float${i}frombits(";
							fi;
							
							p=$(( $startP * $i / 8 - startP));
							shift=0;

							for n in $(seq 1 $(( $i / 8 ))); do
								if [ $n -ne 1 ]; then
									echo -n " | ";
								fi;
								if [ "$i" -ne 8 ]; then
									echo -n "uint$ti(";
								fi;
								echo -n "e.buffer[$p]";
								if [ "$i" -ne 8 ]; then
									echo -n ")";
								fi;
								if [ $shift -gt 0 ]; then
									echo -n "<<$shift";
								fi;
								let "shift += 8";
								let "p += order";
							done;

							if [ "$t" != "Uint" ]; then
								echo -n ")";
							fi;

							if [ -z "$s" ]; then
								echo ", $(( $i / 8 )), nil";
							else
								echo;
							fi;
						else
							var="d";
							if [ $i -eq 8 ]; then
								if [ "$t" = "Int" ]; then
									echo "	e.buffer[0] = byte(d)";
								else
									echo "	e.buffer[0] = d";
								fi;
							else
								if [ "$t" = "Float" ]; then
									var="c";
									echo "	c := math.Float${i}bits(d)";
								elif [ "$t" = "Int" ]; then
									var="c";
									echo "	c := uint${ti}(d)";
								fi;
								echo "	e.buffer = [9]byte{";

								shift=0;

								if [ $order -eq -1 ]; then
									shift=$(( ((i / 8) - 1) * 8));
								fi;

								for n in $(seq $(( i / 8 ))); do
									echo -n "		byte(";
									echo -n "$var";
									if [ $shift -ne 0 ]; then
										echo -n " >> $shift";
									fi;
									echo "),";
									let "shift += 8 * $order";
								done;

								echo "	}";
							fi;

							if [ -z "$s" ]; then
								echo -n "	return ";
							else
								echo "	var n int";
								echo -n "	n, e.Err = ";
							fi;
							
							echo  "e.Writer.Write(e.buffer[:$(( $i / 8 ))])";
							
							if [ ! -z "$s" ]; then
								echo "	e.Count += int64(n)";
							fi;
						fi;

						echo "}";
					done;
				done;
				echo;
				echo "// ${rw}String ${rw}s a string";
				echo -n "func (e *${s}${e}Endian${rw}${er}) ${rw}String(";
				if [ "$rw" = "Write" ]; then
					echo "str string) (int, error) {";
					if [ -z "$s" ]; then
						echo "	return io.WriteString(e.Writer, str)";
					else
						echo "	if e.Err != nil {";
						echo "		return 0, e.Err";
						echo "	}";
						echo "	var n int";
						echo "	n, e.Err = io.WriteString(e.Writer, str)";
						echo "	e.Count += int64(n)";
						echo "	return n, e.Err";
					fi;
				else
					echo -n "size int) ";
					if [ -z "$s" ]; then
						echo "(string, int, error) {";
						echo "	buf := make([]byte, size)";
						echo "	n, err := io.ReadFull(e, buf)";
						echo "	return string(buf[:n]), n, err";
					else
						echo "string {";
						echo "	if e.Err != nil {";
						echo "		return \"\"";
						echo "	}";
						echo "	buf := make([]byte, size)";
						echo "	var n int";
						echo "	n, e.Err = io.ReadFull(e.Reader, buf)";
						echo "	e.Count += int64(n)";
						echo "	return string(buf[:n])";
					fi;
				fi;
				echo "}";
				for size in "X" 8 16 24 32 40 48 56 64; do
					tSize="$size";
					if [ "$size" = "X" ]; then
						tSize="64";
					elif [ $size -eq 24 ]; then
						tSize=32;
					elif [ $size -gt 32 -a $size -lt 64 ]; then
						tSize=64;
					fi;
					echo;
					echo "// ${rw}String${size} ${rw}s the length of the string, using ReadUint${size} and then ${rw}s the bytes of the string";
					echo -n "func (e *${s}${e}Endian${rw}${er}) ${rw}String${size}(";
					if [ "$rw" = "Write" ]; then
						if [ -z "$s" ]; then
							echo "str string) (int, error) {";
							echo "	n, err := e.WriteUint${size}(uint${tSize}(len(str)))";
							echo "	if err != nil {";
							echo "		return n, err";
							echo "	}";
							echo "	m, err := e.WriteString(str)";
							echo "	return n + m, err";
						else
							echo "str string) {";
							echo "	e.WriteUint${size}(uint${tSize}(len(str)))";
							echo "	e.WriteString(str)"
						fi;
					else
						if [ -z "$s" ]; then
							echo ") (string, int, error) {";
							echo "	size, n, err := e.ReadUint${size}()";
							echo "	if err != nil {"
							echo "		return \"\", n, err";
							echo "	}";
							echo "	str, m, err := e.ReadString(int(size))"
							echo "	return str, n + m, err";
						else
							echo ") string {";
							echo "	return e.ReadString(int(e.ReadUint${size}()))";
						fi;
					fi;
					echo "}";
				done;
			) > "$(echo "${s}${e}Endian${rw}${er}" | tr A-Z a-z).go";
		done;
	done;
done;
