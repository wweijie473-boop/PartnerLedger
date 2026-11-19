const base = '/api/tables/v1/sheets'

export function createTableRepository(request) {
  return {
    list: after => request(base + (after ? '?' + new URLSearchParams({ after }) : '')),
    create: settings => request(base, { method: 'POST', body: settings }),
    source(id) {
      const url = base + '/' + encodeURIComponent(id)
      return {
        getSheet: () => request(url),
        getWindow: (bounds, signal) => request(url + '/window?' + new URLSearchParams(bounds), { signal }),
        search: (body, signal) => request(url + '/search', { method: 'POST', body, signal }),
        saveChanges: body => request(url + '/changes', { method: 'POST', body }),
        exportURL: url + '/csv',
      }
    },
  }
}
