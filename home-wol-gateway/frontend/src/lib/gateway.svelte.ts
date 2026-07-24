const URL_KEY = "home-wol-gateway:url";
const TOKEN_KEY = "home-wol-gateway:token";

let url = $state(localStorage.getItem(URL_KEY) ?? "");
let token = $state(localStorage.getItem(TOKEN_KEY) ?? "");

function normalize(value: string): string {
	return value.trim().replace(/\/+$/, "");
}

export const gateway = {
	get url() {
		return url;
	},
	get token() {
		return token;
	},
	set(newUrl: string, newToken: string) {
		url = normalize(newUrl);
		token = newToken.trim();

		if (url) {
			localStorage.setItem(URL_KEY, url);
		} else {
			localStorage.removeItem(URL_KEY);
		}

		if (token) {
			localStorage.setItem(TOKEN_KEY, token);
		} else {
			localStorage.removeItem(TOKEN_KEY);
		}
	},
	clear() {
		url = "";
		token = "";
		localStorage.removeItem(URL_KEY);
		localStorage.removeItem(TOKEN_KEY);
	},
};
