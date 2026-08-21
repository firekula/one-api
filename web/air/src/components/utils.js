import { API, showError } from '../helpers';

export async function getOAuthState() {
  const res = await API.get('/api/oauth/state');
  const { success, message, data } = res.data;
  if (success) {
    return data;
  } else {
    showError(message);
    return '';
  }
}

export async function onGitHubOAuthClicked(github_client_id) {
  const state = await getOAuthState();
  if (!state) return;
  window.open(
    `https://github.com/login/oauth/authorize?client_id=${github_client_id}&state=${state}&scope=user:email`
  );
}

export async function onLarkOAuthClicked(lark_client_id) {
  const state = await getOAuthState();
  if (!state) return;
  let redirect_uri = `${window.location.origin}/oauth/lark`;
  window.open(
    `https://accounts.feishu.cn/open-apis/authen/v1/authorize?client_id=${lark_client_id}&response_type=code&redirect_uri=${redirect_uri}&state=${state}`
  );
}