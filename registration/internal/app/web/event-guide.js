'use strict';
// Uses the existing staff session, CSRF-aware API helper and escaped templates.
async function eventGuidePage() {
  const guide = await api('/admin/event-guide');
  const schemas = {
    sessions: [['title','Session title'],['description','Description','textarea'],['starts_at','Start (India time)','datetime-local'],['ends_at','End (India time)','datetime-local'],['location','Hall / location'],['speakers','Speakers']],
    activities: [['title','Activity title'],['description','Description','textarea'],['schedule','Schedule'],['location','Location']],
    faqs: [['question','Question'],['answer','Answer','textarea']],
    venue: [['address','Full address'],['arrival','Arrival, parking and check-in','textarea'],['accessibility','Accessibility','textarea'],['floor_plan_url','Floor plan HTTPS URL','url'],['help_email','Help desk email','email'],['help_phone','Help desk phone','tel']]
  };
  function localIndia(value) {
    if (!value) return '';
    return new Date(new Date(value).getTime() + 330 * 60000).toISOString().slice(0,16);
  }
  function fields(kind,item) {
    return schemas[kind].map(([key,label,type='text']) => {
      const value = type === 'datetime-local' ? localIndia(item[key]) : item[key] || '';
      return `<label>${esc(label)}${type==='textarea'?`<textarea data-field="${key}" rows="3" maxlength="6000">${esc(value)}</textarea>`:`<input data-field="${key}" type="${type}" value="${esc(value)}" maxlength="${['title','question'].includes(key)?300:6000}" ${['title','question'].includes(key)?'required':''}>`}</label>`;
    }).join('');
  }
  function entry(kind,item) {
    return `<fieldset data-kind="${kind}" data-id="${esc(item.id||'')}"><legend>${esc(item.title||item.question||(kind==='venue'?'Venue details':'New entry'))}</legend><div class="fields">${fields(kind,item)}</div><label><input type="checkbox" data-field="published" ${item.published?'checked':''}> Published in the mobile app</label>${kind==='venue'?'':'<button type="button" data-remove class="secondary">Remove entry</button>'}</fieldset>`;
  }
  app.innerHTML = `<div class="admin-head"><h1>Event guide</h1><a href="/admin">Back to administration</a></div><p>Manage the mobile app’s sessions, activities, FAQs and venue details. Empty sections show “To be announced”. Only published entries appear in the app.</p><p>Session times use India time (IST). Leave both times blank if the schedule is unconfirmed. Changes take effect after saving and the app’s next refresh.</p><form id="event-guide-form"><section class="card"><h2>Venue</h2>${entry('venue',guide.venue)}</section>${['sessions','activities','faqs'].map(kind=>`<section class="card"><h2>${({sessions:'Sessions',activities:'Activities',faqs:'FAQs'})[kind]}</h2><div id="guide-${kind}">${guide[kind].map(item=>entry(kind,item)).join('')}</div><button type="button" data-add="${kind}" class="secondary">Add ${({sessions:'session',activities:'activity',faqs:'FAQ'})[kind]}</button></section>`).join('')}<div class="actions"><button type="submit">Save event guide</button><button type="button" id="reload-guide" class="secondary">Reload saved content</button></div></form>`;
  const form = document.getElementById('event-guide-form');
  let dirty = false;
  form.addEventListener('input',()=>{dirty=true;});
  form.addEventListener('click',e=>{
    const add=e.target.closest('[data-add]');
    if(add){const kind=add.dataset.add;document.getElementById(`guide-${kind}`).insertAdjacentHTML('beforeend',entry(kind,{id:uuid(),published:false}));dirty=true;}
    const remove=e.target.closest('[data-remove]');
    if(remove && confirm('Remove this entry? Save the guide to apply the removal.')){remove.closest('fieldset').remove();dirty=true;}
  });
  bind('reload-guide','click',async()=>{if(!dirty||confirm('Discard unsaved changes and reload?'))await eventGuidePage();});
  bind('event-guide-form','submit',async e=>{
    e.preventDefault();
    const body={revision:guide.revision,sessions:[],activities:[],faqs:[],venue:{}};
    form.querySelectorAll('fieldset[data-kind]').forEach(row=>{
      const kind=row.dataset.kind,item=kind==='venue'?{}:{id:row.dataset.id};
      row.querySelectorAll('[data-field]').forEach(input=>{
        let value=input.type==='checkbox'?input.checked:input.value.trim();
        if(input.type==='datetime-local' && value)value+=':00+05:30';
        item[input.dataset.field]=value;
      });
      if(kind==='venue')body.venue=item;else body[kind].push(item);
    });
    await busy(e.submitter,async()=>{
      const saved=await api('/admin/event-guide',{method:'PUT',body:JSON.stringify(body)});
      guide.revision=saved.revision;dirty=false;message('Event guide saved. Published entries are available to the mobile app.');
    });
  });
}
