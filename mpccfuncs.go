package lnd

/*
	struct Receive_return {
		char* r0;
		int r1;
		char* r2;
	};

	struct Receive_return receive_cgo(char* msg, int length, void* p) {
	   struct Receive_return receive(char* msg, int length, void* p);
	   return receive(msg, length, p);
	}

	char* send_cgo(void* p) {
	   char* send(void* p);
	   return send(p);
	}
*/
import "C"
